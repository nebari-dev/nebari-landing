// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	landingcache "github.com/nebari-dev/nebari-landing/internal/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("websocket")

var upgrader = websocket.Upgrader{
	// Allow all origins -- CORS is handled at the API level.
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

// Hub manages active WebSocket connections and broadcasts events to all of them.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}
}

// NewHub creates a new, ready-to-use Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]struct{}),
	}
}

// Broadcast serialises event and sends it to every connected client.
// Clients that fail to receive are silently dropped.
func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Error(err, "Failed to marshal WebSocket event")
		return
	}

	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		conns = append(conns, c)
	}
	h.mu.RUnlock()

	for _, c := range conns {
		// Set a per-frame deadline so a slow/stuck client cannot block Broadcast
		// indefinitely. The http.Server WriteTimeout is disabled (0) to keep WS
		// connections alive; this deadline replaces it at the message level.
		_ = c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
			log.V(1).Info("WebSocket write failed, dropping client", "error", err)
			h.drop(c)
		}
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

	// Drain incoming frames so the connection stays healthy and we detect
	// client-side closes (ping/pong or explicit close frames).
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
