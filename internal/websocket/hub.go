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
//	                              │  → Broadcast to   │
//	                              │    WS clients     │
//	                              └───────────────────┘
//
// Each webapi replica publishes to Redis and subscribes from Redis, so every
// replica fans out all events to its own connected clients regardless of which
// replica originated the event.
package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/redis/go-redis/v9"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("websocket")

const redisPubSubChannel = "nebari:events"

var upgrader = websocket.Upgrader{
	// Allow all origins — CORS is handled at the Envoy Gateway level.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// EventType represents the kind of service change.
type EventType string

const (
	EventAdded    EventType = "added"
	EventModified EventType = "modified"
	EventDeleted  EventType = "deleted"
)

// Event is the message broadcast to WebSocket clients.
type Event struct {
	Type    EventType                 `json:"type"`
	Service *landingcache.ServiceInfo `json:"service"`
}

// Hub manages active WebSocket connections on this replica and fans out events
// received from the Redis Pub/Sub channel to all connected clients.
type Hub struct {
	rdb *redis.Client
	mu  sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

// NewHub creates a Hub backed by the given Redis client and starts the
// background subscription goroutine. The provided context controls the
// subscription lifetime — cancel it to stop the goroutine cleanly.
func NewHub(ctx context.Context, rdb *redis.Client) *Hub {
	h := &Hub{
		rdb:     rdb,
		clients: make(map[*websocket.Conn]struct{}),
	}
	go h.subscribe(ctx)
	return h
}

// subscribe blocks, receiving messages from the Redis Pub/Sub channel and
// broadcasting them to locally connected WebSocket clients.
func (h *Hub) subscribe(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, redisPubSubChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			h.broadcast([]byte(msg.Payload))
		}
	}
}

// broadcast sends raw JSON to every connected WebSocket client.
// Clients that fail to receive are silently dropped.
func (h *Hub) broadcast(data []byte) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		// Per-frame write deadline prevents a slow/stuck client from blocking.
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			log.V(1).Info("WebSocket write failed, dropping client", "error", err)
			h.drop(c)
		}
	}
}

// Broadcast serialises event, publishes it to Redis (fan-out to all replicas),
// and returns. Local delivery happens via the subscribe goroutine.
func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error(err, "Failed to marshal WebSocket event")
		return
	}
	if err := h.rdb.Publish(context.Background(), redisPubSubChannel, data).Err(); err != nil {
		log.Error(err, "Failed to publish WebSocket event to Redis")
	}
}

// Publish maps a plain string event type to a typed Event and broadcasts it.
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
	h.Broadcast(Event{Type: et, Service: service})
}

// ServeWS upgrades an HTTP connection to WebSocket, registers the client,
// and blocks until the client disconnects.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error(err, "WebSocket upgrade failed")
		return
	}

	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	log.V(1).Info("WebSocket client connected", "remote", r.RemoteAddr)

	// Drain incoming frames to keep the connection healthy and detect
	// client-side closes (ping/pong or close frames).
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	h.drop(conn)
}

// ClientCount returns the number of currently connected clients (useful for tests).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) drop(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
		_ = conn.Close()
		log.V(1).Info("WebSocket client disconnected")
	}
}
