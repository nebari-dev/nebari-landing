// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package websocket_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
)

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

func newServer(t *testing.T, hub *wshub.Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	t.Cleanup(srv.Close)
	return srv
}

func TestNewHub_StartsEmpty(t *testing.T) {
	h := wshub.NewHub()
	if h.ClientCount() != 0 {
		t.Errorf("expected 0 clients, got %d", h.ClientCount())
	}
}

func TestHub_ClientConnectsAndDisconnects(t *testing.T) {
	h := wshub.NewHub()
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
	h := wshub.NewHub()
	srv := newServer(t, h)

	conn := dialWS(t, srv)
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)

	svc := &landingcache.ServiceInfo{Name: "grafana", Namespace: "monitoring"}
	h.Broadcast(wshub.Event{Type: wshub.EventAdded, Service: svc})

	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var evt wshub.Event
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

func TestHub_PublishMapsEventTypes(t *testing.T) {
	tests := []struct {
		input    string
		wantType wshub.EventType
	}{
		{"added", wshub.EventAdded},
		{"modified", wshub.EventModified},
		{"deleted", wshub.EventDeleted},
		{"unknown", wshub.EventModified}, // default
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			h := wshub.NewHub()
			srv := newServer(t, h)

			conn := dialWS(t, srv)
			defer func() { _ = conn.Close() }()
			time.Sleep(20 * time.Millisecond)

			svc := &landingcache.ServiceInfo{Name: "svc"}
			h.Publish(tc.input, svc)

			_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var evt wshub.Event
			if err := json.Unmarshal(msg, &evt); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if evt.Type != tc.wantType {
				t.Errorf("Publish(%q): expected type %q, got %q", tc.input, tc.wantType, evt.Type)
			}
		})
	}
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	h := wshub.NewHub()
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
	h.Broadcast(wshub.Event{Type: wshub.EventModified, Service: svc})

	for i, c := range []*websocket.Conn{conn1, conn2} {
		_ = c.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, msg, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read: %v", i+1, err)
		}
		var evt wshub.Event
		if err := json.Unmarshal(msg, &evt); err != nil {
			t.Fatalf("client %d unmarshal: %v", i+1, err)
		}
		if evt.Service == nil || evt.Service.Name != "multi" {
			t.Errorf("client %d: unexpected service %+v", i+1, evt.Service)
		}
	}
}

func TestHub_BroadcastNoClients_NoError(t *testing.T) {
	h := wshub.NewHub()
	// Should not panic or error
	h.Broadcast(wshub.Event{Type: wshub.EventDeleted, Service: &landingcache.ServiceInfo{Name: "gone"}})
}
