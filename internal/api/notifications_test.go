// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
)

// newNotifStore creates a miniredis-backed notification store for tests.
func newNotifStore(t *testing.T) *notifications.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return notifications.NewStore(rdb)
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
	h := &Handler{}
	if !h.hasRequiredGroups(nil, nil) {
		t.Error("empty requiredGroups should return true")
	}
}

func TestHasRequiredGroups_UserHasGroup_ReturnsTrue(t *testing.T) {
	h := &Handler{}
	if !h.hasRequiredGroups([]string{"devs", "admins"}, []string{"admins"}) {
		t.Error("user has required group — expected true")
	}
}

func TestHasRequiredGroups_UserMissingGroup_ReturnsFalse(t *testing.T) {
	h := &Handler{}
	if h.hasRequiredGroups([]string{"devs"}, []string{"admins"}) {
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
