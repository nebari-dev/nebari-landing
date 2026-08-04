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
	"github.com/golang-jwt/jwt/v5"
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

// futureExp returns a jwt.NumericDate pointer one hour in the future.
// Tests that exercise the WS upgrade path need a valid exp because handleWS
// rejects tickets whose snapshotted ExpiresAt is zero or already past.
func futureExp() *jwt.NumericDate {
	return jwt.NewNumericDate(time.Now().Add(time.Hour))
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
	ticket, err := store.Issue(context.Background(), wsticket.TicketClaims{Subject: "test-user", ExpiresAt: time.Now().Add(time.Hour)})
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
	ticket, err := store.Issue(context.Background(), wsticket.TicketClaims{Subject: "test-user", ExpiresAt: time.Now().Add(time.Hour)})
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
	ticket, err := store.Issue(context.Background(), wsticket.TicketClaims{Subject: "test-user", ExpiresAt: time.Now().Add(time.Hour)})
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

// TestHandleWS_Unauthorized_WithTicketCarryingExpiredJWTExp guards against
// the unbounded-session bypass identified in PR review B1: a JWT can be
// near-exp at ticket-issue time and already past-exp by the time the ticket
// is redeemed. handleWS must reject upgrades whose snapshotted ExpiresAt is
// zero or in the past, because the hub's time.AfterFunc skips installation
// for ttl ≤ 0 and the session would otherwise stay open indefinitely.
func TestHandleWS_Unauthorized_WithTicketCarryingExpiredJWTExp(t *testing.T) {
	store, _ := newTicketStore(t)
	hub := newWSHub(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			if r.Header.Get("Authorization") == "Bearer near-exp" {
				return &auth.Claims{
					PreferredUsername: "alice",
					Groups:            []string{"admin"},
					RegisteredClaims: jwt.RegisteredClaims{
						// Already expired — production JWT validator would
						// reject this on the Bearer path, but the issue is
						// that the ticket value is a snapshot taken at issue
						// time; the redeem-time check is what guards the WS
						// upgrade.
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
					},
				}, true
			}
			return nil, false
		}),
	)
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	// Issue the ticket with the past-exp claims (test bypasses real JWT
	// validation via WithClaimsExtractor).
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer near-exp")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ticket request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from ws-ticket, got %d", resp.StatusCode)
	}
	var body WSTicketResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Upgrade attempt must fail — the snapshotted exp is in the past.
	_, wsResp, err := dialWS(t, srv, "/api/v1/ws?ticket="+body.Ticket, nil)
	if err == nil {
		t.Fatal("expected upgrade to fail for ticket with past exp")
	}
	if wsResp == nil || wsResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", wsResp)
	}
}

// TestHandleWS_Unauthorized_WithTicketCarryingZeroExp covers the second leg
// of the staleness bypass: a TicketClaims with a zero-value ExpiresAt (no
// exp claim on the JWT) would let the hub skip the timer entirely. handleWS
// rejects this at the upgrade.
func TestHandleWS_Unauthorized_WithTicketCarryingZeroExp(t *testing.T) {
	store, _ := newTicketStore(t)
	hub := newWSHub(t)
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			// No RegisteredClaims.ExpiresAt → nil → expFromClaims returns zero.
			return &auth.Claims{PreferredUsername: "alice"}, true
		}),
	)
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer any")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("ticket request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body WSTicketResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	_, wsResp, err := dialWS(t, srv, "/api/v1/ws?ticket="+body.Ticket, nil)
	if err == nil {
		t.Fatal("expected upgrade to fail for ticket with zero exp")
	}
	if wsResp == nil || wsResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", wsResp)
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
				return &auth.Claims{
					PreferredUsername: "alice",
					RegisteredClaims:  jwt.RegisteredClaims{ExpiresAt: futureExp()},
				}, true
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
				return &auth.Claims{
					PreferredUsername: "bob",
					RegisteredClaims:  jwt.RegisteredClaims{ExpiresAt: futureExp()},
				}, true
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
// Ticket → claims → per-client filter, end-to-end (issue #95)
// ═══════════════════════════════════════════════════════════════════

// TestWSTicketExchange_FullFlow_FiltersBroadcastByGroups is the regression
// test for issue #95. It drives a complete production-shape path:
//
//  1. POST /api/v1/ws-ticket with a Bearer token whose claims include
//     groups=[admin]. The handler stores the claims into wsticket.
//  2. Upgrade WS using ?ticket=<id>. The handler redeems the ticket, builds
//     a Principal from the snapshot, and registers the client on the hub.
//  3. hub.SetAccessPolicy(handler) wires the canAccessPolicy rule the REST
//     path enforces.
//  4. Publish a private service requiring "admin". The entitled client
//     receives the frame.
//  5. A second client, registered with groups=[data-science], must NOT
//     receive the frame — the per-client filter drops it before write.
//
// If `handleWS` ever regresses to discarding claims on the ticket path, the
// data-science client will receive the admin-only event and this test fails.
func TestWSTicketExchange_FullFlow_FiltersBroadcastByGroups(t *testing.T) {
	store, _ := newTicketStore(t)
	hub := newWSHub(t)

	// Both legitimate test users share one extractor; the test selects which
	// identity to issue under by switching the Authorization header.
	h := NewHandler(cache.NewServiceCache(), nil, true, hub, nil,
		WithWSTicketStore(store),
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			switch r.Header.Get("Authorization") {
			case "Bearer alice-admin":
				return &auth.Claims{
					PreferredUsername: "alice",
					Groups:            []string{"admin"},
					RegisteredClaims:  jwt.RegisteredClaims{ExpiresAt: futureExp()},
				}, true
			case "Bearer bob-ds":
				return &auth.Claims{
					PreferredUsername: "bob",
					Groups:            []string{"data-science"},
					RegisteredClaims:  jwt.RegisteredClaims{ExpiresAt: futureExp()},
				}, true
			}
			return nil, false
		}),
	)
	// Wire the handler as the policy: production parity, and the actual
	// regression check.
	hub.SetAccessPolicy(h)

	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	// Helper: POST /ws-ticket as a given user, return the ticket id.
	issueTicket := func(t *testing.T, authHeader string) string {
		t.Helper()
		req, _ := http.NewRequestWithContext(context.Background(),
			http.MethodPost, srv.URL+"/api/v1/ws-ticket", nil)
		req.Header.Set("Authorization", authHeader)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("ticket request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 from ws-ticket, got %d", resp.StatusCode)
		}
		var body WSTicketResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode ticket: %v", err)
		}
		if body.Ticket == "" {
			t.Fatal("empty ticket")
		}
		return body.Ticket
	}

	aliceTicket := issueTicket(t, "Bearer alice-admin")
	bobTicket := issueTicket(t, "Bearer bob-ds")

	aliceConn, _, err := dialWS(t, srv, "/api/v1/ws?ticket="+aliceTicket, nil)
	if err != nil {
		t.Fatalf("alice WS upgrade: %v", err)
	}
	defer func() { _ = aliceConn.Close() }()
	bobConn, _, err := dialWS(t, srv, "/api/v1/ws?ticket="+bobTicket, nil)
	if err != nil {
		t.Fatalf("bob WS upgrade: %v", err)
	}
	defer func() { _ = bobConn.Close() }()
	time.Sleep(30 * time.Millisecond)
	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}

	// Publish an admin-only private service.
	hub.Publish("added", &cache.ServiceInfo{
		Name:           "grafana",
		UID:            "g-1",
		Visibility:     "private",
		RequiredGroups: []string{"admin"},
	})

	// Alice (admin) receives the frame, with the right UID.
	_ = aliceConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := aliceConn.ReadMessage()
	if err != nil {
		t.Fatalf("alice read: %v", err)
	}
	var evt wshub.ServiceEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("alice unmarshal: %v", err)
	}
	if evt.Service == nil || evt.Service.UID != "g-1" {
		t.Errorf("alice got unexpected service: %+v", evt.Service)
	}

	// Bob (data-science) MUST NOT receive the frame.
	_ = bobConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, leak, err := bobConn.ReadMessage(); err == nil {
		t.Fatalf("bob received an admin-only frame — claims propagation regressed: %s", leak)
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
