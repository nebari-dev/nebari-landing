// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
	"github.com/nebari-dev/nebari-landing/internal/wsticket"
	"github.com/redis/go-redis/v9"
)

// --- helpers ---

func newTicketStore(t *testing.T) (*wsticket.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return wsticket.NewStore(rdb), mr
}

func newWSHub(t *testing.T) *wshub.Hub {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { _ = rdb.Close() })
	return wshub.NewHub(ctx, rdb)
}

func dialWS(t *testing.T, srv *httptest.Server, path string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	return websocket.DefaultDialer.Dial(wsURL, headers)
}

// ═══════════════════════════════════════════════════════════════════
// POST /api/v1/ws-ticket
// ═══════════════════════════════════════════════════════════════════

func TestHandleWSTicket_MethodNotAllowed(t *testing.T) {
	store, _ := newTicketStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, false, newWSHub(t), nil,
		WithWSTicketStore(store),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws-ticket", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleWSTicket_NotImplemented_WhenNoStore(t *testing.T) {
	// Hub is set (so the route is registered) but wsTicketStore is nil.
	h := NewHandler(cache.NewServiceCache(), nil, false, newWSHub(t), nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ws-ticket", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandleWSTicket_Unauthorized_WhenAuthFails(t *testing.T) {
	store, _ := newTicketStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, newWSHub(t), nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return nil, false // always reject
		}),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ws-ticket", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestHandleWSTicket_ReturnsTicket_WhenAuthenticated(t *testing.T) {
	store, _ := newTicketStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, newWSHub(t), nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{PreferredUsername: "alice"}, true
		}),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ws-ticket", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	var resp WSTicketResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Ticket == "" {
		t.Error("expected non-empty ticket in response")
	}
	if len(resp.Ticket) != 32 {
		t.Errorf("expected 32-char hex ticket, got %q", resp.Ticket)
	}
}

func TestHandleWSTicket_AuthDisabled_ReturnsTicket(t *testing.T) {
	// When auth is disabled, requireAuth returns (_anonymous, true).
	// The ticket endpoint should still issue a ticket.
	store, _ := newTicketStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, false, newWSHub(t), nil,
		WithWSTicketStore(store),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ws-ticket", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp WSTicketResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Ticket == "" {
		t.Error("expected non-empty ticket")
	}
}

func TestHandleWSTicket_EachCallIssuesDifferentTicket(t *testing.T) {
	store, _ := newTicketStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, newWSHub(t), nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{PreferredUsername: "alice"}, true
		}),
	)
	doPost := func() string {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/ws-ticket", nil)
		rr := httptest.NewRecorder()
		h.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		var resp WSTicketResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		return resp.Ticket
	}
	t1, t2 := doPost(), doPost()
	if t1 == t2 {
		t.Error("expected two ticket requests to return different tickets")
	}
}

// ═══════════════════════════════════════════════════════════════════
// GET /api/v1/ws — ticket-based auth path
// ═══════════════════════════════════════════════════════════════════

// handlerWithTicketAuth returns a Handler where:
//   - auth is enabled (enableAuth=true)
//   - no jwtValidator (so Bearer path requires the real validator)
//   - wsTicketStore is wired up (so ticket path works)
//
// This lets us test the ticket path without a real Keycloak.
func handlerWithTicketAuth(t *testing.T, store *wsticket.Store) *Handler {
	t.Helper()
	return NewHandler(cache.NewServiceCache(), nil, true, newWSHub(t), nil,
		WithWSTicketStore(store),
	)
}

func TestHandleWS_Unauthorized_WhenNoCredentials(t *testing.T) {
	store, _ := newTicketStore(t)
	srv := httptest.NewServer(handlerWithTicketAuth(t, store).Routes())
	defer srv.Close()

	_, resp, err := dialWS(t, srv, "/api/v1/ws", nil)
	if err == nil {
		t.Fatal("expected dial to fail with no credentials")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestHandleWS_Upgrades_WithValidTicket(t *testing.T) {
	store, _ := newTicketStore(t)
	ticket, err := store.Issue(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(handlerWithTicketAuth(t, store).Routes())
	defer srv.Close()

	conn, resp, err := dialWS(t, srv, "/api/v1/ws?ticket="+ticket, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("expected upgrade, got status %d, error: %v", status, err)
	}
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond) // let ServeWS register the client
}

func TestHandleWS_Unauthorized_WithInvalidTicket(t *testing.T) {
	store, _ := newTicketStore(t)
	srv := httptest.NewServer(handlerWithTicketAuth(t, store).Routes())
	defer srv.Close()

	_, resp, err := dialWS(t, srv, "/api/v1/ws?ticket=00000000000000000000000000000000", nil)
	if err == nil {
		t.Fatal("expected dial to fail with unknown ticket")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}
}

func TestHandleWS_Unauthorized_WithAlreadyUsedTicket(t *testing.T) {
	store, _ := newTicketStore(t)
	ticket, err := store.Issue(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(handlerWithTicketAuth(t, store).Routes())
	defer srv.Close()

	// First connection — should succeed.
	conn1, _, err := dialWS(t, srv, "/api/v1/ws?ticket="+ticket, nil)
	if err != nil {
		t.Fatalf("first connection failed: %v", err)
	}
	defer func() { _ = conn1.Close() }()
	time.Sleep(20 * time.Millisecond)

	// Second connection with the same ticket — must fail.
	_, resp, err := dialWS(t, srv, "/api/v1/ws?ticket="+ticket, nil)
	if err == nil {
		t.Fatal("expected second connection with used ticket to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on reused ticket, got %v", resp)
	}
}

func TestHandleWS_Unauthorized_WithExpiredTicket(t *testing.T) {
	store, mr := newTicketStore(t)
	ticket, err := store.Issue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(31e9) // jump past the 30 s TTL

	srv := httptest.NewServer(handlerWithTicketAuth(t, store).Routes())
	defer srv.Close()

	_, resp, err := dialWS(t, srv, "/api/v1/ws?ticket="+ticket, nil)
	if err == nil {
		t.Fatal("expected dial to fail with expired ticket")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired ticket, got %v", resp)
	}
}

func TestHandleWS_TicketIgnored_WhenAuthDisabled(t *testing.T) {
	// With auth disabled the WS endpoint must upgrade regardless of ticket presence.
	h := NewHandler(cache.NewServiceCache(), nil, false, newWSHub(t), nil)
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	conn, _, err := dialWS(t, srv, "/api/v1/ws", nil)
	if err != nil {
		t.Fatalf("expected upgrade when auth is disabled, got: %v", err)
	}
	defer func() { _ = conn.Close() }()
	time.Sleep(20 * time.Millisecond)
}

// ═══════════════════════════════════════════════════════════════════
// Full ticket exchange flow: POST /ws-ticket → GET /ws?ticket=
// ═══════════════════════════════════════════════════════════════════

func TestWSTicketExchange_FullFlow_IssueViaHTTP_ThenUpgradeWS(t *testing.T) {
	store, _ := newTicketStore(t)
	hub := newWSHub(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			if r.Header.Get("Authorization") == "Bearer alice-token" {
				return &auth.Claims{PreferredUsername: "alice"}, true
			}
			return nil, false
		}),
	)

	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	// Step 1: obtain a ticket via POST /api/v1/ws-ticket (authenticated Bearer).
	ticketReq, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, srv.URL+"/api/v1/ws-ticket", nil)
	ticketReq.Header.Set("Authorization", "Bearer alice-token")
	ticketResp, err := http.DefaultClient.Do(ticketReq)
	if err != nil {
		t.Fatalf("ticket request failed: %v", err)
	}
	defer func() { _ = ticketResp.Body.Close() }()
	if ticketResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from ws-ticket, got %d", ticketResp.StatusCode)
	}
	var ticketBody WSTicketResponse
	if err := json.NewDecoder(ticketResp.Body).Decode(&ticketBody); err != nil {
		t.Fatalf("failed to decode ticket response: %v", err)
	}
	if ticketBody.Ticket == "" {
		t.Fatal("received empty ticket")
	}

	// Step 2: upgrade the WebSocket using the ticket — no Bearer header sent.
	conn, wsResp, err := dialWS(t, srv, "/api/v1/ws?ticket="+ticketBody.Ticket, nil)
	if err != nil {
		status := 0
		if wsResp != nil {
			status = wsResp.StatusCode
		}
		t.Fatalf("WS upgrade with ticket failed, status %d: %v", status, err)
	}
	defer func() { _ = conn.Close() }()

	// Give the hub time to register the connection.
	time.Sleep(20 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 connected WS client, got %d", hub.ClientCount())
	}
}

func TestWSTicketExchange_TicketIsNotReusable_AfterSuccessfulUpgrade(t *testing.T) {
	store, _ := newTicketStore(t)
	hub := newWSHub(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			if r.Header.Get("Authorization") == "Bearer test" {
				return &auth.Claims{PreferredUsername: "bob"}, true
			}
			return nil, false
		}),
	)
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	// Issue ticket via HTTP.
	req, _ := http.NewRequestWithContext(context.Background(),
		http.MethodPost, srv.URL+"/api/v1/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer test")
	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()
	var body WSTicketResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)

	// First WS upgrade succeeds.
	conn1, _, err := dialWS(t, srv, "/api/v1/ws?ticket="+body.Ticket, nil)
	if err != nil {
		t.Fatalf("first upgrade failed: %v", err)
	}
	defer func() { _ = conn1.Close() }()
	time.Sleep(20 * time.Millisecond)

	// Second upgrade with the same ticket must be rejected.
	_, resp2, err := dialWS(t, srv, "/api/v1/ws?ticket="+body.Ticket, nil)
	if err == nil {
		t.Fatal("expected second upgrade with reused ticket to fail")
	}
	if resp2 == nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on ticket reuse, got %v", resp2)
	}
}

// ═══════════════════════════════════════════════════════════════════
// WithWSTicketStore option
// ═══════════════════════════════════════════════════════════════════

func TestWithWSTicketStore_SetsField(t *testing.T) {
	store, _ := newTicketStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, false, nil, nil,
		WithWSTicketStore(store),
	)
	if h.wsTicketStore != store {
		t.Error("expected wsTicketStore to be set by WithWSTicketStore")
	}
}

func TestWSTicketRoute_NotRegistered_WhenHubIsNil(t *testing.T) {
	store, _ := newTicketStore(t)
	// hub is nil → /api/v1/ws-ticket must not be registered.
	h := NewHandler(cache.NewServiceCache(), nil, false, nil, nil,
		WithWSTicketStore(store),
	)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ws-ticket", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 when hub is nil, got %d", rr.Code)
	}
}
