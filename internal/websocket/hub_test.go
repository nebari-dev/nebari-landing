// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package websocket_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
	"github.com/redis/go-redis/v9"
)

// newTestHub creates a Hub backed by a miniredis instance for testing.
// The hub and miniredis server are both cleaned up via t.Cleanup.
func newTestHub(t *testing.T) *wshub.Hub {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = rdb.Close() })
	return wshub.NewHub(ctx, rdb)
}

// dialWS connects to a test WebSocket server and returns the connection.
func dialWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// newServer wraps ServeWS for tests that don't care about the per-client
// principal or session-end. Tests that need to exercise the access-policy
// filter use newServerWithPrincipal instead.
func newServer(t *testing.T, hub *wshub.Hub) *httptest.Server {
	t.Helper()
	return newServerWithPrincipal(t, hub, wshub.Principal{}, time.Time{})
}

// newServerWithPrincipal serves WS upgrades with a fixed Principal — the
// production wiring derives it from JWT or ticket; here we inject it.
func newServerWithPrincipal(t *testing.T, hub *wshub.Hub, p wshub.Principal, sessionEnd time.Time) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r, p, sessionEnd)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// waitForClients polls h.ClientCount() until it reaches `want` or the
// deadline fires. Used in place of fixed time.Sleep after dialWS, which is
// fragile under -race (5×) on a contended CI runner.
func waitForClients(t *testing.T, h *wshub.Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if h.ClientCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected ClientCount=%d within deadline, got %d", want, h.ClientCount())
}

// groupsPolicy is a minimal ServiceAccessPolicy used by the filter tests.
// Mirrors api.canAccessPolicy: public services are always allowed; private
// services require an authenticated client whose Groups intersect
// service.RequiredGroups.
type groupsPolicy struct{}

func (groupsPolicy) CanAccessService(svc *landingcache.ServiceInfo, p wshub.Principal) bool {
	if svc.Visibility == "public" {
		return true
	}
	if !p.Authenticated {
		return false
	}
	if len(svc.RequiredGroups) == 0 {
		return true
	}
	for _, want := range svc.RequiredGroups {
		for _, have := range p.Groups {
			if want == have {
				return true
			}
		}
	}
	return false
}

func TestNewHub_StartsEmpty(t *testing.T) {
	h := newTestHub(t)
	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.ClientCount())
	}
}

func TestHub_ClientConnectsAndDisconnects(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn := dialWS(t, srv)

	// Give ServeWS goroutine time to register the client.
	time.Sleep(20 * time.Millisecond)
	if h.ClientCount() != 1 {
		t.Errorf("expected 1 client after connect, got %d", h.ClientCount())
	}

	_ = conn.Close()
	time.Sleep(50 * time.Millisecond)
	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", h.ClientCount())
	}
}

func TestHub_BroadcastDeliveredToClient(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	svc := &landingcache.ServiceInfo{Name: "grafana", Namespace: "monitoring"}
	h.Publish("added", svc)

	// Allow time for Redis Pub/Sub round-trip + local broadcast.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var evt wshub.ServiceEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != wshub.EventAdded {
		t.Errorf("expected type %q, got %q", wshub.EventAdded, evt.Type)
	}
	if evt.Service == nil || evt.Service.Name != "grafana" {
		t.Errorf("unexpected service: %+v", evt.Service)
	}
}

// --- Publish event-type mapping ---

func publishAndReadType(t *testing.T, input string) wshub.EventType {
	t.Helper()
	h := newTestHub(t)
	srv := newServer(t, h)
	conn := dialWS(t, srv)
	t.Cleanup(func() { _ = conn.Close() })
	time.Sleep(20 * time.Millisecond)

	h.Publish(input, &landingcache.ServiceInfo{Name: "svc"})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var evt wshub.ServiceEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return evt.Type
}

func TestHub_Publish_Added_MapsToEventAdded(t *testing.T) {
	if got := publishAndReadType(t, "added"); got != wshub.EventAdded {
		t.Errorf("expected %q, got %q", wshub.EventAdded, got)
	}
}

func TestHub_Publish_Modified_MapsToEventModified(t *testing.T) {
	if got := publishAndReadType(t, "modified"); got != wshub.EventModified {
		t.Errorf("expected %q, got %q", wshub.EventModified, got)
	}
}

func TestHub_Publish_Deleted_MapsToEventDeleted(t *testing.T) {
	if got := publishAndReadType(t, "deleted"); got != wshub.EventDeleted {
		t.Errorf("expected %q, got %q", wshub.EventDeleted, got)
	}
}

func TestHub_Publish_Unknown_DefaultsToEventModified(t *testing.T) {
	if got := publishAndReadType(t, "unknown"); got != wshub.EventModified {
		t.Errorf("expected %q (default), got %q", wshub.EventModified, got)
	}
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn1 := dialWS(t, srv)
	conn2 := dialWS(t, srv)
	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()
	time.Sleep(30 * time.Millisecond)

	if h.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", h.ClientCount())
	}

	svc := &landingcache.ServiceInfo{Name: "multi"}
	h.Publish("modified", svc)

	for i, c := range []*websocket.Conn{conn1, conn2} {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read: %v", i+1, err)
		}
		var evt wshub.ServiceEvent
		if err := json.Unmarshal(msg, &evt); err != nil {
			t.Fatalf("client %d unmarshal: %v", i+1, err)
		}
		if evt.Service == nil || evt.Service.Name != "multi" {
			t.Errorf("client %d: unexpected service %+v", i+1, evt.Service)
		}
	}
}

func TestHub_BroadcastNoClients_NoError(t *testing.T) {
	h := newTestHub(t)
	// Should not panic or error
	h.Publish("deleted", &landingcache.ServiceInfo{Name: "gone"})
}

func TestHub_BroadcastPayload_ServiceEvent_HasCorrectSchema(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	svc := &landingcache.ServiceInfo{Name: "jupyter", Namespace: "default", UID: "abc-123"}
	h.Publish("added", svc)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("broadcast is not valid JSON: %v", err)
	}
	for _, key := range []string{"type", "service"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("broadcast payload missing required field %q; full payload: %s", key, raw)
		}
	}
	var evtType string
	if err := json.Unmarshal(payload["type"], &evtType); err != nil || evtType == "" {
		t.Errorf("payload[type] is not a non-empty string: %s", payload["type"])
	}
	var svcObj map[string]json.RawMessage
	if err := json.Unmarshal(payload["service"], &svcObj); err != nil {
		t.Fatalf("payload[service] is not a JSON object: %v", err)
	}
	for _, key := range []string{"name", "uid"} {
		if _, ok := svcObj[key]; !ok {
			t.Errorf("service object missing required field %q", key)
		}
	}
}

func TestHub_BroadcastPayload_NotificationEvent_HasCorrectSchema(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	n := &notifications.Notification{ID: "notif-42", Title: "Test", Message: "Body"}
	h.PublishNotification(n)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("broadcast is not valid JSON: %v", err)
	}
	for _, key := range []string{"type", "notification"} {
		if _, ok := payload[key]; !ok {
			t.Errorf("broadcast payload missing required field %q; full payload: %s", key, raw)
		}
	}
	var notifObj map[string]json.RawMessage
	if err := json.Unmarshal(payload["notification"], &notifObj); err != nil {
		t.Fatalf("payload[notification] is not a JSON object: %v", err)
	}
	for _, key := range []string{"id", "title", "message"} {
		if _, ok := notifObj[key]; !ok {
			t.Errorf("notification object missing required field %q", key)
		}
	}
}

func TestHub_ClientDisconnectMidBroadcast_NoDeadlock(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn := dialWS(t, srv)
	time.Sleep(20 * time.Millisecond)

	_ = conn.Close()
	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		h.Publish("added", &landingcache.ServiceInfo{Name: "svc"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Publish blocked or deadlocked after client disconnect")
	}

	time.Sleep(50 * time.Millisecond)
	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", h.ClientCount())
	}
}

func TestHub_PublishNotification_DeliveredToClient(t *testing.T) {
	h := newTestHub(t)
	srv := newServer(t, h)

	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	n := &notifications.Notification{
		ID:      "notif-123",
		Title:   "Scheduled maintenance",
		Message: "The cluster will restart at midnight.",
	}
	h.PublishNotification(n)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var evt wshub.NotificationEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type != wshub.EventNotificationCreate {
		t.Errorf("expected type %q, got %q", wshub.EventNotificationCreate, evt.Type)
	}
	if evt.Notification == nil {
		t.Fatal("Notification field is nil")
	}
	if evt.Notification.ID != n.ID {
		t.Errorf("expected notification ID %q, got %q", n.ID, evt.Notification.ID)
	}
	if evt.Notification.Title != n.Title {
		t.Errorf("expected title %q, got %q", n.Title, evt.Notification.Title)
	}
}

// --- Per-client access filter (issue #95) ---

// TestHub_BroadcastService_FilteredByPolicy_OnlyEntitledClientReceives drives
// a private service publish through the real Redis-subscribe → dispatch →
// broadcastService chain with two clients whose Principals differ in group
// membership. The policy passes only one of them; the other must NOT see the
// frame and must read-deadline-expire.
//
// This is the regression case for #95.
func TestHub_BroadcastService_FilteredByPolicy_OnlyEntitledClientReceives(t *testing.T) {
	h := newTestHub(t)
	h.SetAccessPolicy(groupsPolicy{})

	srvAdmin := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "alice", Groups: []string{"admin"}, Authenticated: true},
		time.Time{})
	srvDS := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "bob", Groups: []string{"data-science"}, Authenticated: true},
		time.Time{})

	connAdmin := dialWS(t, srvAdmin)
	defer func() { _ = connAdmin.Close() }()
	connDS := dialWS(t, srvDS)
	defer func() { _ = connDS.Close() }()
	waitForClients(t, h, 2)

	// First publish a PUBLIC service. Both clients must receive it — this
	// proves the Redis-Sub → dispatch → broadcastService pipeline is flushed
	// end-to-end at this moment in the test. We then use that fact to bound
	// the negative-receive assertion on the private publish below: if the
	// pipeline delivered the public event, it would have delivered the
	// private one too if the filter were broken.
	probe := &landingcache.ServiceInfo{Name: "probe", UID: "p-1", Visibility: "public"}
	h.Publish("added", probe)
	for i, c := range []*websocket.Conn{connAdmin, connDS} {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := c.ReadMessage(); err != nil {
			t.Fatalf("client %d did not receive the public probe frame: %v", i+1, err)
		}
	}

	// Now the actual case: a private service requiring "admin".
	svc := &landingcache.ServiceInfo{
		Name: "grafana", UID: "g-1",
		Visibility: "private", RequiredGroups: []string{"admin"},
	}
	h.Publish("added", svc)

	// Entitled client receives the frame with the actual service UID.
	_ = connAdmin.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := connAdmin.ReadMessage()
	if err != nil {
		t.Fatalf("admin client read: %v", err)
	}
	var evt wshub.ServiceEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("admin client unmarshal: %v", err)
	}
	if evt.Service == nil || evt.Service.UID != "g-1" {
		t.Errorf("admin client got unexpected service: %+v", evt.Service)
	}

	// Non-entitled client must NOT receive the frame. We synchronized on
	// the public probe above, so 500 ms here is plenty of slack — if the
	// filter were broken the frame would already be in flight.
	_ = connDS.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, leak, err := connDS.ReadMessage()
	if err == nil {
		t.Fatalf("data-science client received a frame for a private admin-only service: %s", leak)
	}
}

// TestHub_BroadcastNotification_AndService_IndependentFanOut locks in the
// invariant that notifications bypass the per-client filter while service
// events honor it. With two principals (admin, data-science) and a denying
// policy on the private service, publishing the notification reaches both;
// publishing the private service reaches only admin. Catches a regression
// where the two fan-out paths get cross-wired.
func TestHub_BroadcastNotification_AndService_IndependentFanOut(t *testing.T) {
	h := newTestHub(t)
	h.SetAccessPolicy(groupsPolicy{})

	srvAdmin := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "alice", Groups: []string{"admin"}, Authenticated: true},
		time.Time{})
	srvDS := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "bob", Groups: []string{"data-science"}, Authenticated: true},
		time.Time{})

	connAdmin := dialWS(t, srvAdmin)
	defer func() { _ = connAdmin.Close() }()
	connDS := dialWS(t, srvDS)
	defer func() { _ = connDS.Close() }()
	waitForClients(t, h, 2)

	// Notification — both clients must receive.
	h.PublishNotification(&notifications.Notification{ID: "n-1", Title: "outage", Message: "."})
	for i, c := range []*websocket.Conn{connAdmin, connDS} {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, raw, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("client %d did not receive the notification: %v", i+1, err)
		}
		var ev wshub.NotificationEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatalf("client %d unmarshal notification: %v", i+1, err)
		}
		if ev.Notification == nil || ev.Notification.ID != "n-1" {
			t.Errorf("client %d got wrong notification: %+v", i+1, ev.Notification)
		}
	}

	// Service — admin-only private — reaches admin, not data-science.
	h.Publish("added", &landingcache.ServiceInfo{
		Name: "grafana", UID: "g-1",
		Visibility: "private", RequiredGroups: []string{"admin"},
	})
	_ = connAdmin.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := connAdmin.ReadMessage(); err != nil {
		t.Fatalf("admin did not receive private service: %v", err)
	}
	_ = connDS.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, leak, err := connDS.ReadMessage(); err == nil {
		t.Fatalf("data-science received an admin-only frame: %s", leak)
	}
}

// TestHub_BroadcastService_AnonymousPrincipal_PublicOnly covers the auth-
// disabled / unauthenticated path. handleWS builds a zero-value Principal
// in that case (Authenticated=false). The policy must let public events
// through and block private ones — same rule REST applies to the same
// caller.
func TestHub_BroadcastService_AnonymousPrincipal_PublicOnly(t *testing.T) {
	h := newTestHub(t)
	h.SetAccessPolicy(groupsPolicy{})

	srv := newServerWithPrincipal(t, h, wshub.Principal{}, time.Time{})
	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	waitForClients(t, h, 1)

	// Public service — must reach the anonymous client.
	h.Publish("added", &landingcache.ServiceInfo{Name: "public-svc", UID: "p-1", Visibility: "public"})
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("anonymous client did not receive public service: %v", err)
	}

	// Private service — must NOT reach anonymous.
	h.Publish("added", &landingcache.ServiceInfo{
		Name: "private-svc", UID: "p-2",
		Visibility: "private", RequiredGroups: []string{"admin"},
	})
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, leak, err := conn.ReadMessage(); err == nil {
		t.Fatalf("anonymous client received a private frame: %s", leak)
	}
}

// TestHub_BroadcastService_NilPolicy_AllowsAll preserves the test-only
// back-compat: when no policy is set, every service event reaches every
// client (existing tests above implicitly rely on this).
func TestHub_BroadcastService_NilPolicy_AllowsAll(t *testing.T) {
	h := newTestHub(t)
	// No SetAccessPolicy call.

	srv := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "bob", Groups: []string{"data-science"}, Authenticated: true},
		time.Time{})
	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	// Private service the principal would normally fail the filter on.
	svc := &landingcache.ServiceInfo{
		Name: "secret", Visibility: "private", RequiredGroups: []string{"admin"},
	}
	h.Publish("added", svc)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("expected frame to reach client when policy is nil, got: %v", err)
	}
}

// TestHub_BroadcastNotification_BypassesPolicy ensures notification events
// fan out to every client regardless of the configured ServiceAccessPolicy —
// the policy only applies to ServiceEvent. Filtering notifications is a
// deferred follow-up (#95 out-of-scope).
func TestHub_BroadcastNotification_BypassesPolicy(t *testing.T) {
	h := newTestHub(t)
	// A policy that would deny everything if consulted.
	h.SetAccessPolicy(denyAllPolicy{})

	srv := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "anyone", Authenticated: true},
		time.Time{})
	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	h.PublishNotification(&notifications.Notification{ID: "n1", Title: "hi", Message: "."})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("notification should bypass policy, got: %v", err)
	}
}

type denyAllPolicy struct{}

func (denyAllPolicy) CanAccessService(*landingcache.ServiceInfo, wshub.Principal) bool {
	return false
}

// TestHub_SessionEnd_ClosesConn schedules an immediate session-end and
// asserts the hub closes the connection by way of the per-connection timer
// installed in ServeWS. SPA reconnects with a fresh ticket in production;
// the test just verifies the close fires.
func TestHub_SessionEnd_ClosesConn(t *testing.T) {
	h := newTestHub(t)
	srv := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "alice", Authenticated: true},
		time.Now().Add(50*time.Millisecond))

	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	waitForClients(t, h, 1)

	// The hub's timer should close the underlying connection. ReadMessage
	// returns an error on close — give it generous slack to fire.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected ReadMessage to error after session-end close, got nil")
	}
	waitForClients(t, h, 0)
}

// TestHub_ConcurrentConnectsAndPublishes_NoRace stresses the per-client state
// added for the filter under `-race`. N clients connect concurrently, the
// hub publishes service events from another goroutine, and per-client drain
// goroutines count what each socket actually received. Any racy mutation
// under the snapshot lock would trip the detector here; any regression that
// drops every event would be caught by the per-client receive assertion
// (assertion-independence — the assertion does not derive from the loop
// under test).
func TestHub_ConcurrentConnectsAndPublishes_NoRace(t *testing.T) {
	h := newTestHub(t)
	h.SetAccessPolicy(groupsPolicy{})
	srv := newServerWithPrincipal(t, h,
		wshub.Principal{Subject: "u", Groups: []string{"admin"}, Authenticated: true},
		time.Time{})

	const n = 8
	conns := make([]*websocket.Conn, n)
	var dialWG sync.WaitGroup
	for i := range n {
		dialWG.Add(1)
		go func(i int) {
			defer dialWG.Done()
			url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
			c, _, err := websocket.DefaultDialer.Dial(url, nil)
			if err != nil {
				t.Errorf("client %d dial: %v", i, err)
				return
			}
			conns[i] = c
		}(i)
	}
	dialWG.Wait()
	t.Cleanup(func() {
		for _, c := range conns {
			if c != nil {
				_ = c.Close()
			}
		}
	})
	waitForClients(t, h, n)

	// Per-client drain goroutines — count frames each socket actually
	// received. Independent of the publish-side counter below.
	received := make([]atomic.Int32, n)
	var drainWG sync.WaitGroup
	for i, c := range conns {
		if c == nil {
			continue
		}
		drainWG.Add(1)
		go func(i int, c *websocket.Conn) {
			defer drainWG.Done()
			for {
				_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
				received[i].Add(1)
			}
		}(i, c)
	}

	// Publisher.
	const events = 20
	var published atomic.Int32
	for range events {
		h.Publish("modified", &landingcache.ServiceInfo{
			Name: "svc", Visibility: "private", RequiredGroups: []string{"admin"},
		})
		published.Add(1)
		time.Sleep(2 * time.Millisecond)
	}

	// Give the fan-out a moment, then close all conns to release drainWG.
	time.Sleep(200 * time.Millisecond)
	for _, c := range conns {
		if c != nil {
			_ = c.Close()
		}
	}
	drainWG.Wait()

	if got := published.Load(); got != events {
		t.Errorf("expected %d publishes, got %d", events, got)
	}
	for i := range n {
		if r := received[i].Load(); r == 0 {
			t.Errorf("client %d received 0 frames out of %d publishes — regression where fan-out is dropping all events would land here", i, events)
		}
	}
}
