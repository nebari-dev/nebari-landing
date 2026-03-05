// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/cache"
)

// newARStore creates a miniredis-backed access request store for tests.
func newARStore(t *testing.T) *accessrequests.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return accessrequests.NewStore(rdb)
}

// newARHandler returns a Handler with auth disabled and an access request store wired in.
func newARHandler(sc *cache.ServiceCache, store *accessrequests.Store) *Handler {
	return NewHandler(sc, nil, false, nil, nil, WithAccessRequestStore(store))
}

// --- POST /api/v1/services/{id}/request_access ---

func TestHandleRequestAccess_NoStore_Returns501(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access", nil)
	rr := httptest.NewRecorder()
	newTestHandler(sc).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandleRequestAccess_MethodNotAllowed(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/uid-pub/request_access", nil)
	rr := httptest.NewRecorder()
	newARHandler(sc, newARStore(t)).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleRequestAccess_ServiceNotFound(t *testing.T) {
	// auth is disabled → requireAuth returns _anonymous; service not in cache → 404
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/unknown-uid/request_access", nil)
	rr := httptest.NewRecorder()
	newARHandler(cache.NewServiceCache(), newARStore(t)).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleRequestAccess_Success_Returns202(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access",
		strings.NewReader(`{"message":"please"}`))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(`{"message":"please"}`))
	rr := httptest.NewRecorder()
	newARHandler(sc, newARStore(t)).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var ar accessrequests.AccessRequest
	if err := json.NewDecoder(rr.Body).Decode(&ar); err != nil {
		t.Fatal(err)
	}
	if ar.ID == "" {
		t.Error("expected non-empty ID in response")
	}
	if ar.Status != accessrequests.StatusPending {
		t.Errorf("expected pending, got %q", ar.Status)
	}
}

func TestHandleRequestAccess_DuplicatePending_Returns409(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	store := newARStore(t)
	h := newARHandler(sc, store)

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access", nil)
		rr := httptest.NewRecorder()
		h.Routes().ServeHTTP(rr, req)
		return rr
	}

	if rr := post(); rr.Code != http.StatusAccepted {
		t.Fatalf("first request: expected 202, got %d", rr.Code)
	}
	if rr := post(); rr.Code != http.StatusConflict {
		t.Errorf("duplicate request: expected 409, got %d", rr.Code)
	}
}

func TestHandleRequestAccess_NoBody_StillAccepted(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access", nil)
	rr := httptest.NewRecorder()
	newARHandler(sc, newARStore(t)).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202 with no body, got %d", rr.Code)
	}
}

// --- GET /api/v1/admin/access-requests ---

func TestHandleAdminListAccessRequests_NoStore_Returns501(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/admin/access-requests")
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandleAdminListAccessRequests_NotAdmin_Returns403(t *testing.T) {
	// Even with enableAuth=true, when jwtValidator is nil requireAuth returns the
	// anonymous claims (not 401). The anonymous user has no groups, so isAdmin→false→403.
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	store := newARStore(t)
	h := NewHandler(sc, nil, true, nil, nil, WithAccessRequestStore(store))
	rr := doGet(t, h.Routes(), "/api/v1/admin/access-requests")
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestHandleAdminListAccessRequests_AuthDisabled_NoAdminGroup_Returns403(t *testing.T) {
	// Auth disabled: requireAuth returns _anonymous with no groups → isAdmin → false → 403
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	store := newARStore(t)
	h := newARHandler(sc, store)
	rr := doGet(t, h.Routes(), "/api/v1/admin/access-requests")
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 (_anonymous has no admin group), got %d", rr.Code)
	}
}

func TestHandleAdminListAccessRequests_MethodNotAllowed(t *testing.T) {
	store := newARStore(t)
	// POST to admin list endpoint — method check happens before admin check → 405.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/access-requests", nil)
	rr := httptest.NewRecorder()
	newARHandler(cache.NewServiceCache(), store).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- PUT /api/v1/admin/access-requests/{id}/approve|deny ---

func TestHandleAdminAccessRequestSub_NoStore_Returns501(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/access-requests/some-id/approve", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

// With a store configured, anonymous callers (no admin group) get 403 before path dispatch.
// Full path/action routing is covered by store unit tests.
func TestHandleAdminAccessRequestSub_AnonymousCaller_Returns403(t *testing.T) {
	store := newARStore(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/access-requests/some-id/approve", nil)
	rr := httptest.NewRecorder()
	newARHandler(cache.NewServiceCache(), store).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 (anonymous → no admin group), got %d", rr.Code)
	}
}
