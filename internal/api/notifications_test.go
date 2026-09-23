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
	"github.com/redis/go-redis/v9"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
)

// newNotifStore creates a miniredis-backed notification store for tests.
func newNotifStore(t *testing.T) *notifications.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return notifications.NewStore(rdb, time.Hour)
}

// newNotifHandler wires a notification store into a handler with auth disabled.
func newNotifHandler(sc *cache.ServiceCache, store *notifications.Store) *Handler {
	return NewHandler(sc, nil, false, nil, nil, WithNotificationStore(store))
}

// --- WithAdminGroup / WithNotificationStore / WithKeycloakAdminClient ---

func TestWithAdminGroup_SetsField(t *testing.T) {
	h := NewHandler(cache.NewServiceCache(), nil, false, nil, nil, WithAdminGroup("superusers"))
	if h.adminGroup != "superusers" {
		t.Errorf("expected adminGroup=superusers, got %q", h.adminGroup)
	}
}

func TestWithNotificationStore_SetsField(t *testing.T) {
	s := newNotifStore(t)
	h := NewHandler(cache.NewServiceCache(), nil, false, nil, nil, WithNotificationStore(s))
	if h.notificationStore != s {
		t.Error("expected notificationStore to be set by WithNotificationStore")
	}
}

func TestWithKeycloakAdminClient_NilAccepted(t *testing.T) {
	// nil is a valid value; verify the option is applied without panic.
	h := NewHandler(cache.NewServiceCache(), nil, false, nil, nil, WithKeycloakAdminClient(nil))
	if h.keycloakClient != nil {
		t.Error("expected keycloakClient to be nil")
	}
}

// --- hasRequiredGroups ---

func TestHasRequiredGroups_EmptyRequired_ReturnsTrue(t *testing.T) {
	if !hasRequiredGroups(nil, nil) {
		t.Error("empty requiredGroups should return true")
	}
}

func TestHasRequiredGroups_UserHasGroup_ReturnsTrue(t *testing.T) {
	if !hasRequiredGroups([]string{"devs", "admins"}, []string{"admins"}) {
		t.Error("user has required group — expected true")
	}
}

func TestHasRequiredGroups_UserMissingGroup_ReturnsFalse(t *testing.T) {
	if hasRequiredGroups([]string{"devs"}, []string{"admins"}) {
		t.Error("user is missing required group — expected false")
	}
}

// --- applyKeycloakGroupMembership ---

func TestApplyKeycloakGroupMembership_NilClient_DoesNotPanic(t *testing.T) {
	// When keycloakClient is nil the function must return immediately without panicking.
	h := &Handler{cache: cache.NewServiceCache()}
	req := &accessrequests.AccessRequest{
		UserID:      "user1",
		ServiceUID:  "svc1",
		ServiceName: "my-service",
	}
	h.applyKeycloakGroupMembership(context.Background(), req)
}

// --- PUT /api/v1/notifications/{id}/read ---

func TestHandleNotificationSub_NoStore_Returns501(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/some-id/read", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandleNotificationSub_MethodNotAllowed(t *testing.T) {
	store := newNotifStore(t)
	h := newNotifHandler(cache.NewServiceCache(), store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/some-id/read", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleNotificationSub_InvalidPath_NoAction_Returns400(t *testing.T) {
	store := newNotifStore(t)
	h := newNotifHandler(cache.NewServiceCache(), store)
	// Only one path segment after the prefix — no action part → 400.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/only-id-no-action", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleNotificationSub_UnknownAction_Returns404(t *testing.T) {
	store := newNotifStore(t)
	h := newNotifHandler(cache.NewServiceCache(), store)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/some-id/archive", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown action, got %d", rr.Code)
	}
}

func TestHandleNotificationSub_NotificationNotFound_Returns404(t *testing.T) {
	store := newNotifStore(t)
	h := newNotifHandler(cache.NewServiceCache(), store)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/does-not-exist/read", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent notification, got %d", rr.Code)
	}
}

func TestHandleNotificationSub_MarkRead_Returns204(t *testing.T) {
	store := newNotifStore(t)
	n, err := store.Create("", "Hello", "World")
	if err != nil {
		t.Fatal(err)
	}
	h := newNotifHandler(cache.NewServiceCache(), store)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/"+n.ID+"/read", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d — body: %s", rr.Code, rr.Body.String())
	}
}

func TestNotificationReadStateUsesStableSubjectNotUsername(t *testing.T) {
	store := newNotifStore(t)
	n, err := store.Create("", "Hello", "World")
	if err != nil {
		t.Fatal(err)
	}

	claims := claimsWithIdentity("https://keycloak.example/realms/main", "subject-1", "alice")
	h := newNotifHandlerWithClaims(cache.NewServiceCache(), store,
		func(_ *http.Request) (*auth.Claims, bool) { return claims, true })

	req := httptest.NewRequest(http.MethodPut, "/api/v1/notifications/"+n.ID+"/read", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("mark read: expected 204, got %d: %s", rr.Code, rr.Body.String())
	}

	claims = claimsWithIdentity("https://keycloak.example/realms/main", "subject-1", "alice-renamed")
	rr = doGet(t, h.Routes(), "/api/v1/notifications")
	if rr.Code != http.StatusOK {
		t.Fatalf("renamed user: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var renamed struct {
		Notifications []NotificationItem `json:"notifications"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&renamed); err != nil {
		t.Fatal(err)
	}
	if len(renamed.Notifications) != 1 || !renamed.Notifications[0].Read {
		t.Fatalf("renamed subject should keep read state, got %+v", renamed.Notifications)
	}

	claims = claimsWithIdentity("https://keycloak.example/realms/main", "subject-2", "alice")
	rr = doGet(t, h.Routes(), "/api/v1/notifications")
	if rr.Code != http.StatusOK {
		t.Fatalf("reused username: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var reused struct {
		Notifications []NotificationItem `json:"notifications"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&reused); err != nil {
		t.Fatal(err)
	}
	if len(reused.Notifications) != 1 || reused.Notifications[0].Read {
		t.Fatalf("reused username must not inherit read state, got %+v", reused.Notifications)
	}
}

// --- POST /api/v1/admin/notifications ---

func TestHandleAdminCreateNotification_NoStore_Returns501(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications",
		strings.NewReader(`{"title":"t","message":"m"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandleAdminCreateNotification_MethodNotAllowed_Returns405(t *testing.T) {
	store := newNotifStore(t)
	h := newNotifHandler(cache.NewServiceCache(), store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/notifications", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleAdminCreateNotification_NotAdmin_Returns403(t *testing.T) {
	// Anonymous caller (_anonymous) has no groups → isAdmin returns false → 403.
	store := newNotifStore(t)
	h := newNotifHandler(cache.NewServiceCache(), store)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications",
		strings.NewReader(`{"title":"t","message":"m"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

// TestHandleAdminCreateNotification_PublishesToHub verifies that a successful
// POST /api/v1/admin/notifications pushes a notification.created WebSocket
// frame to all connected hub clients in addition to persisting the record.
func TestHandleAdminCreateNotification_PublishesToHub(t *testing.T) {
	// Stand up a miniredis-backed Hub.
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hub := wshub.NewHub(ctx, rdb)

	// Connect a WebSocket client to the hub before the POST. The handler in
	// production passes a Principal + session-end; this test exercises the
	// notification fan-out which is policy-agnostic, so a zero Principal is
	// fine.
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r, wshub.Principal{}, time.Time{})
	}))
	t.Cleanup(wsSrv.Close)
	wsURL := "ws" + strings.TrimPrefix(wsSrv.URL, "http") + "/"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial hub WS: %v", err)
	}
	defer func() { _ = wsConn.Close() }()
	time.Sleep(20 * time.Millisecond) // wait for registration

	// Wire a notification store and the hub into the handler.
	store := newNotifStore(t)
	h := NewHandler(
		cache.NewServiceCache(), nil, false, hub, nil,
		WithNotificationStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{PreferredUsername: "admin-user", Groups: []string{"admin"}}, true
		}),
	)

	// POST a new notification.
	body := strings.NewReader(`{"title":"Maintenance window","message":"Cluster restart at midnight"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/notifications", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d — body: %s", rr.Code, rr.Body.String())
	}

	// The WS client should receive a notification.created frame.
	_ = wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("read WS message: %v", err)
	}

	var frame wshub.NotificationEvent
	if err := json.Unmarshal(msg, &frame); err != nil {
		t.Fatalf("unmarshal WS frame: %v", err)
	}
	if frame.Type != "notification.created" {
		t.Errorf("expected type %q, got %q", "notification.created", frame.Type)
	}
	if frame.Notification == nil {
		t.Fatal("Notification field is nil in WS frame")
	}
	if frame.Notification.Title != "Maintenance window" {
		t.Errorf("unexpected title %q", frame.Notification.Title)
	}
}

// --- GET /api/v1/notifications: per-caller filter (issue #170) ---

// newNotifHandlerWithClaims wires a handler with auth enabled, a claims
// extractor (so we can simulate authenticated / anonymous / bad-token
// callers without a live JWT validator), and a notification store.
func newNotifHandlerWithClaims(sc *cache.ServiceCache, store *notifications.Store, extractor func(*http.Request) (*auth.Claims, bool)) *Handler {
	return NewHandler(sc, nil, true, nil, nil,
		WithNotificationStore(store),
		WithClaimsExtractor(extractor),
	)
}

// listNotifs runs GET /api/v1/notifications through the handler and returns
// the decoded titles in response order (newest first per the store).
func listNotifs(t *testing.T, h *Handler) (int, []string) {
	t.Helper()
	rr := doGet(t, h.Routes(), "/api/v1/notifications")
	if rr.Code != http.StatusOK {
		return rr.Code, nil
	}
	var body struct {
		Notifications []NotificationItem `json:"notifications"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	titles := make([]string, 0, len(body.Notifications))
	for _, n := range body.Notifications {
		titles = append(titles, n.Title)
	}
	return rr.Code, titles
}

// seedNotifs writes three notifications covering the three access shapes:
// untagged (broadcast), tagged-public (visible to all), and tagged-private
// with a required group (visible only to callers in that group). Titles are
// used by the assertions.
func seedNotifs(t *testing.T, s *notifications.Store) {
	t.Helper()
	if _, err := s.CreateDraft(notifications.Draft{Title: "welcome"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDraft(notifications.Draft{Title: "public-lifecycle", ServiceUID: "svc-pub", Visibility: "public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDraft(notifications.Draft{Title: "private-lifecycle", ServiceUID: "svc-priv", Visibility: "private", RequiredGroups: []string{"argocd-admins"}}); err != nil {
		t.Fatal(err)
	}
}

func TestHandleGetNotifications_MemberInGroup_SeesAll(t *testing.T) {
	store := newNotifStore(t)
	seedNotifs(t, store)
	claims := &auth.Claims{PreferredUsername: "admin", Groups: []string{"argocd-admins"}}
	h := newNotifHandlerWithClaims(cache.NewServiceCache(), store,
		func(_ *http.Request) (*auth.Claims, bool) { return claims, true })
	code, titles := listNotifs(t, h)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(titles) != 3 {
		t.Errorf("group member should see all 3 notifications, got %v", titles)
	}
}

func TestHandleGetNotifications_NonMember_PrivateFiltered(t *testing.T) {
	// This is the #170 regression guard: Alice was in argocd-viewers only,
	// received the private-service lifecycle notification because REST list
	// had no filter, and the SPA rendered it in her feed even though the
	// service card itself was correctly hidden.
	store := newNotifStore(t)
	seedNotifs(t, store)
	claims := &auth.Claims{PreferredUsername: "alice", Groups: []string{"argocd-viewers"}}
	h := newNotifHandlerWithClaims(cache.NewServiceCache(), store,
		func(_ *http.Request) (*auth.Claims, bool) { return claims, true })
	code, titles := listNotifs(t, h)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	got := map[string]bool{}
	for _, tt := range titles {
		got[tt] = true
	}
	if got["private-lifecycle"] {
		t.Errorf("non-member alice must NOT see private-lifecycle; got titles=%v", titles)
	}
	if !got["welcome"] || !got["public-lifecycle"] {
		t.Errorf("alice should still see untagged + public; got titles=%v", titles)
	}
}

func TestHandleGetNotifications_Anonymous_OnlyPublicAndUntagged(t *testing.T) {
	// Anonymous callers (no valid claims) may only see notifications that
	// canAccessPolicy would allow: untagged broadcasts and tagged-public.
	// Tagged-private is authenticated-only regardless of RequiredGroups.
	store := newNotifStore(t)
	seedNotifs(t, store)
	h := newNotifHandlerWithClaims(cache.NewServiceCache(), store,
		func(_ *http.Request) (*auth.Claims, bool) { return nil, false })
	code, titles := listNotifs(t, h)
	if code != http.StatusOK {
		t.Fatalf("expected 200 for anon, got %d", code)
	}
	got := map[string]bool{}
	for _, tt := range titles {
		got[tt] = true
	}
	if got["private-lifecycle"] {
		t.Errorf("anon must NOT see private notifications; got titles=%v", titles)
	}
	if !got["welcome"] || !got["public-lifecycle"] {
		t.Errorf("anon should see untagged + public; got titles=%v", titles)
	}
}

func TestHandleGetNotifications_AuthFailure_DoesNotLeakList(t *testing.T) {
	// Issue #166: handleGetNotifications previously called requireAuth,
	// which writes a 401 for a bad token but the handler fell through and
	// then encoded the full list into the same response. This test locks
	// in that the handler stops at the auth-failure point and never emits
	// the list body afterwards.
	//
	// The new implementation uses extractAndValidateJWT (not requireAuth),
	// which returns authenticated=false without writing a 401 for bad-token
	// cases. Bad-token callers are then treated exactly like anonymous
	// callers: they get untagged + tagged-public only, never tagged-private.
	// That behavior is asserted here.
	store := newNotifStore(t)
	seedNotifs(t, store)
	// Simulate a bad token: extractor reports "not ok" (as extractAndValidateJWT
	// does for an invalid Bearer). Handler must treat as anonymous, not leak
	// private notifications.
	h := newNotifHandlerWithClaims(cache.NewServiceCache(), store,
		func(_ *http.Request) (*auth.Claims, bool) { return nil, false })
	code, titles := listNotifs(t, h)
	if code != http.StatusOK {
		t.Fatalf("expected 200 (bad-token treated as anon), got %d", code)
	}
	for _, tt := range titles {
		if tt == "private-lifecycle" {
			t.Errorf("bad-token caller must not receive tagged-private notifications; got %v", titles)
		}
	}
}
