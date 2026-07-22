// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package websocket provides the Hub that manages WebSocket client connections
// and broadcasts service-change events in real time.
//
// Fan-out architecture:
//
//	┌─────────────┐   Publish()   ┌───────────────────┐
//	│ NebariApp   │ ────────────► │  Hub.Publish()    │
//	│  watcher    │               │  → PUBLISH to     │
//	└─────────────┘               │    Redis channel  │
//	                              └────────┬──────────┘
//	                                       │ Redis Pub/Sub
//	                              ┌────────▼──────────┐
//	                              │  Hub.subscribe()  │
//	                              │  goroutine        │
//	                              │  → broadcastService /
//	                              │    broadcastAll   │
//	                              └───────────────────┘
//
// Each webapi replica publishes to Redis and subscribes from Redis, so every
// replica fans out all events to its own connected clients regardless of which
// replica originated the event.
//
// # Per-client access filter
//
// Service-change events are filtered per connected client via the
// ServiceAccessPolicy supplied through SetAccessPolicy. The handler that owns
// the per-request policy (`api.Handler.CanAccessService`) is the natural
// implementer; the hub stays identity-agnostic by taking a small Principal
// value type rather than importing the JWT package. NotificationEvent applies
// the same policy for notifications tied to a source service (ServiceUID set)
// by synthesizing a ServiceInfo carrying the source service's Visibility and
// RequiredGroups; untagged notifications (admin broadcasts) still fan out to
// every connected client.
//
// Conventions
//
//   - `client` fields are immutable after registration. The broadcast loop
//     snapshots the client set under RLock, then evaluates the policy and
//     writes outside the lock. Mutating a registered client's principal would
//     race the broadcast goroutine — instead, drop and re-register.
//   - Hub-side logs name the caller by opaque subject (`subject`) only, never
//     by preferred_username, email, or group membership. Future contributors:
//     keep PII out of these logs.
package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
	"github.com/redis/go-redis/v9"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("websocket")

const redisPubSubChannel = "nebari:events"

// writeTimeout bounds how long a single frame write may block a slow client
// before the hub gives up and drops them. The deadline is per-frame, not
// per-connection — a healthy client can stay connected for the lifetime of
// the session.
const writeTimeout = 10 * time.Second

var upgrader = websocket.Upgrader{
	// Allow all origins — CORS is handled at the Envoy Gateway level.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// EventType is the value carried in the "type" field of every WebSocket frame.
type EventType string

const (
	EventAdded              EventType = "added"
	EventModified           EventType = "modified"
	EventDeleted            EventType = "deleted"
	EventNotificationCreate EventType = "notification.created"
)

// ServiceEvent is the WebSocket frame sent when a NebariApp service is added,
// modified, or deleted. Type is one of EventAdded / EventModified / EventDeleted.
type ServiceEvent struct {
	Type    EventType                 `json:"type"`
	Service *landingcache.ServiceInfo `json:"service"`
}

// NotificationEvent is the WebSocket frame sent when a new platform-wide
// notification is created. Type is always EventNotificationCreate.
type NotificationEvent struct {
	Type         EventType                   `json:"type"`
	Notification *notifications.Notification `json:"notification"`
}

// Principal is the per-client identity the hub applies the access policy
// against. It is deliberately a narrow projection of `auth.Claims` — the hub
// must not learn the shape of the JWT or anything the policy does not need.
type Principal struct {
	// Subject is the opaque user identifier (JWT `sub`). Use this for logging;
	// never log PreferredUsername / email / groups from hub code.
	Subject string
	// Groups are the user's group memberships used by the access policy.
	Groups []string
	// Authenticated mirrors the boolean the REST path passes to canAccessService.
	// False means anonymous; the policy gives anonymous clients public services
	// only.
	Authenticated bool
}

// ServiceAccessPolicy decides whether a connected client may receive
// service-change events for a given service. Implemented by the API handler
// so REST and WebSocket apply the same `canAccessService` rules.
type ServiceAccessPolicy interface {
	CanAccessService(service *landingcache.ServiceInfo, p Principal) bool
}

// client is a single WebSocket connection plus its access-policy state.
// Fields are set at registration time and never mutated; see the package
// comment for the rationale.
type client struct {
	conn      *websocket.Conn
	principal Principal
}

// Hub manages active WebSocket connections on this replica and fans out events
// received from the Redis Pub/Sub channel to all connected clients.
type Hub struct {
	rdb     *redis.Client
	mu      sync.RWMutex
	clients map[*client]struct{}
	// policy is consulted on every ServiceEvent broadcast. A nil policy means
	// "allow all" — preserved for tests that build hubs directly. Production
	// wiring in cmd/main.go always sets a non-nil policy.
	policy ServiceAccessPolicy
	// nilPolicyWarn fires once per Hub lifetime the first time a service event
	// is broadcast while policy is still nil. The setter pattern means a
	// future call site could forget SetAccessPolicy and silently reintroduce
	// the issue #95 fan-out-to-all regression; this turns the silent failure
	// into a noisy one without breaking the test-time allow-all convenience.
	nilPolicyWarn sync.Once
}

// NewHub creates a Hub backed by the given Redis client and starts the
// background subscription goroutine. The provided context controls the
// subscription lifetime — cancel it to stop the goroutine cleanly.
//
// The hub starts without an access policy. Call SetAccessPolicy after the
// handler that implements ServiceAccessPolicy is constructed.
func NewHub(ctx context.Context, rdb *redis.Client) *Hub {
	h := &Hub{
		rdb:     rdb,
		clients: make(map[*client]struct{}),
	}
	go h.subscribe(ctx)
	return h
}

// SetAccessPolicy installs the per-client filter applied to service-change
// events. Wired in cmd/main.go after both the hub and the handler exist —
// the handler is constructed after the hub, so this setter is the join point.
// Safe to call before any clients connect; not intended for live swaps under
// load.
func (h *Hub) SetAccessPolicy(p ServiceAccessPolicy) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.policy = p
}

// subscribe blocks, receiving messages from the Redis Pub/Sub channel and
// dispatching them to the appropriate broadcast path.
func (h *Hub) subscribe(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, redisPubSubChannel)
	defer func() { _ = pubsub.Close() }()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.dispatch([]byte(msg.Payload))
		}
	}
}

// dispatch routes a published frame to the right fan-out path based on its
// "type" field. Service events go through the per-client filter;
// notifications and unknown event types fan out to everyone.
func (h *Hub) dispatch(payload []byte) {
	var env struct {
		Type EventType `json:"type"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		// Malformed envelope: drop and log at Info so an operator can spot a
		// publisher that started emitting bad bytes without flooding Error.
		log.Info("WebSocket: malformed event envelope, dropping",
			"error", err.Error())
		return
	}
	switch env.Type {
	case EventAdded, EventModified, EventDeleted:
		var ev ServiceEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			log.Info("WebSocket: malformed service event, dropping",
				"error", err.Error())
			return
		}
		if ev.Service == nil {
			// Defensive: a publisher emitting a service-typed envelope with
			// no `service` field would cause every policy implementation to
			// nil-deref `svc.Visibility`. Drop and log.
			log.Info("WebSocket: service event with nil Service field, dropping",
				"type", ev.Type)
			return
		}
		h.broadcastService(&ev, payload)
	case EventNotificationCreate:
		var ev NotificationEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			log.Info("WebSocket: malformed notification event, dropping",
				"error", err.Error())
			return
		}
		if ev.Notification == nil {
			log.Info("WebSocket: notification event with nil Notification field, dropping")
			return
		}
		h.broadcastNotification(&ev, payload)
	default:
		// Forward-compat unknown types: fan out to all so a newer publisher
		// stays deliverable through an older hub.
		h.broadcastAll(payload)
	}
}

// broadcastService delivers a service-change event to every connected client
// whose Principal passes the access policy. The Service field on the event
// drives the filter; the wire bytes (`raw`) are written verbatim so the SPA
// sees the same payload format as a non-filtered fan-out would have produced.
func (h *Hub) broadcastService(ev *ServiceEvent, raw []byte) {
	h.mu.RLock()
	policy := h.policy
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	if policy == nil {
		h.nilPolicyWarn.Do(func() {
			log.Info("WebSocket: no access policy set — service events will fan out to every client; call Hub.SetAccessPolicy in production wiring (issue #95 regression risk)")
		})
	}

	for _, c := range clients {
		if policy != nil && !policy.CanAccessService(ev.Service, c.principal) {
			continue
		}
		h.writeOrDrop(c, raw)
	}
}

// broadcastNotification delivers a notification-created event with the same
// per-client access filtering the service-event path applies. Notifications
// tied to a source service (ServiceUID set) are gated by the shared
// ServiceAccessPolicy against a synthesized ServiceInfo carrying the source
// service's Visibility and RequiredGroups; untagged notifications (admin
// broadcasts, welcome messages) fan out to every connected client.
//
// The synthesized ServiceInfo is enough because the policy consumes only
// Visibility and RequiredGroups — cheaper than plumbing a second policy
// method and keeps the "REST and WS see the same decision" invariant.
func (h *Hub) broadcastNotification(ev *NotificationEvent, raw []byte) {
	h.mu.RLock()
	policy := h.policy
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	// Untagged notifications: fan out to all. No policy consultation needed.
	if ev.Notification.ServiceUID == "" {
		for _, c := range clients {
			h.writeOrDrop(c, raw)
		}
		return
	}

	if policy == nil {
		h.nilPolicyWarn.Do(func() {
			log.Info("WebSocket: no access policy set — notification events will fan out to every client; call Hub.SetAccessPolicy in production wiring (issue #170 regression risk)")
		})
	}

	proxy := &landingcache.ServiceInfo{
		UID:            ev.Notification.ServiceUID,
		Visibility:     ev.Notification.Visibility,
		RequiredGroups: ev.Notification.RequiredGroups,
	}
	for _, c := range clients {
		if policy != nil && !policy.CanAccessService(proxy, c.principal) {
			continue
		}
		h.writeOrDrop(c, raw)
	}
}

// broadcastAll delivers a frame to every connected client unconditionally.
// Used for forward-compat unknown event types only.
func (h *Hub) broadcastAll(raw []byte) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		h.writeOrDrop(c, raw)
	}
}

// writeOrDrop applies the per-frame deadline and drops the client on write
// failure. Sits outside the RLock so a slow client cannot block subsequent
// writes longer than writeTimeout.
func (h *Hub) writeOrDrop(c *client, raw []byte) {
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := c.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
		log.V(1).Info("WebSocket write failed, dropping client", "error", err)
		h.drop(c)
	}
}

// publish marshals v and publishes the bytes to the Redis Pub/Sub channel.
// Local delivery happens via the subscribe goroutine that every replica runs.
func (h *Hub) publish(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Error(err, "Failed to marshal WebSocket event")
		return
	}
	if err := h.rdb.Publish(context.Background(), redisPubSubChannel, data).Err(); err != nil {
		// Downgraded from Error to Info: publish failures are transient Redis
		// infrastructure errors that generate noisy stack traces at Error level.
		log.Info("WebSocket publish failed (transient Redis error)", "error", err)
	}
}

// Publish broadcasts a service-change event. eventType must be one of
// "added", "modified", or "deleted"; unknown values default to "modified".
// The watcher calls this so it does not need to import this package directly.
func (h *Hub) Publish(eventType string, service *landingcache.ServiceInfo) {
	var et EventType
	switch eventType {
	case "added":
		et = EventAdded
	case "modified":
		et = EventModified
	case "deleted":
		et = EventDeleted
	default:
		et = EventModified
	}
	h.publish(ServiceEvent{Type: et, Service: service})
}

// PublishNotification broadcasts a notification-created event to all connected
// WebSocket clients via the shared Redis pub/sub channel.
func (h *Hub) PublishNotification(n *notifications.Notification) {
	h.publish(NotificationEvent{Type: EventNotificationCreate, Notification: n})
}

// ServeWS upgrades an HTTP connection to WebSocket, registers the client with
// the supplied Principal, and blocks until the client disconnects.
//
// sessionEnd bounds the server-side lifetime of this connection: if non-zero
// and in the future, the hub schedules a close at that wall-clock time so a
// stale claims snapshot cannot outlive the underlying JWT. The SPA must
// reconnect (re-validating the JWT via a fresh ticket) when this fires.
// Pass time.Time{} (zero) to skip the timer — appropriate for tests, for
// auth-disabled dev mode, or for principal sources without an explicit exp.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, principal Principal, sessionEnd time.Time) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err, "WebSocket upgrade failed")
		return
	}

	c := &client{conn: conn, principal: principal}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	log.V(1).Info("WebSocket client connected",
		"remote", r.RemoteAddr, "subject", principal.Subject)

	// Bound the session at the JWT exp. Close from a timer is safe even if
	// the connection is already torn down — the drain loop below catches the
	// resulting read error and falls through to drop().
	if !sessionEnd.IsZero() {
		ttl := time.Until(sessionEnd)
		if ttl > 0 {
			t := time.AfterFunc(ttl, func() {
				log.V(1).Info("WebSocket session expired, forcing reconnect",
					"subject", principal.Subject)
				_ = conn.Close()
			})
			defer t.Stop()
		}
	}

	// Drain incoming frames to keep the connection healthy and detect
	// client-side closes (ping/pong or close frames).
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	h.drop(c)
}

// ClientCount returns the number of currently connected clients (useful for tests).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) drop(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		_ = c.conn.Close()
		log.V(1).Info("WebSocket client disconnected",
			"subject", c.principal.Subject)
	}
}
