// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	wshub "github.com/nebari-dev/nebari-landing/internal/websocket"
)

// newARStore creates a miniredis-backed access request store for tests.
func newARStore(t *testing.T) *accessrequests.Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return accessrequests.NewStore(rdb)
}

func resolveAccessRequestForTest(t *testing.T, store *accessrequests.Store, id string, resolution accessrequests.Resolution) *accessrequests.AccessRequest {
	t.Helper()
	updated, err := store.Resolve(id, resolution)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return updated
}

// newARHandler returns a Handler with auth disabled and an access request store wired in.
func newARHandler(sc *cache.ServiceCache, store *accessrequests.Store) *Handler {
	return NewHandler(sc, nil, false, nil, nil, WithAccessRequestStore(store))
}

func addPrivateService(sc *cache.ServiceCache, uid, name string, requiredGroups []string) {
	sc.Add(&sdapp.App{
		UID:        uid,
		Name:       name,
		Namespace:  "default",
		Hostname:   name + ".example.com",
		TLSEnabled: true,
		LandingPage: &sdapp.LandingPage{
			Enabled:        true,
			DisplayName:    name,
			Visibility:     "private",
			RequiredGroups: requiredGroups,
		},
	})
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

func TestHandleRequestAccess_StoresStableSubject(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	store := newARStore(t)
	h := NewHandler(sc, nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				PreferredUsername: "alice",
				Email:             "alice@example.com",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "user-subject"},
			}, true
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var ar accessrequests.AccessRequest
	if err := json.NewDecoder(rr.Body).Decode(&ar); err != nil {
		t.Fatal(err)
	}
	if ar.UserID != "user-subject" {
		t.Fatalf("expected stable subject in access request, got %q", ar.UserID)
	}
	if ar.TargetOwner != "default/pub" {
		t.Fatalf("expected target owner default/pub, got %q", ar.TargetOwner)
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

func TestHandleRequestAccess_ActiveEntitlement_Returns409(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	store := newARStore(t)
	ar, err := store.Create("uid-pub", "pub", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:    accessrequests.StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(sc, nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				PreferredUsername: "alice",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "user-subject"},
			}, true
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409 for active entitlement, got %d — body: %s", rr.Code, rr.Body.String())
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

func TestHandleAdminListAccessRequests_StatusFiltersResolvedRequests(t *testing.T) {
	store := newARStore(t)
	approved, _ := store.Create("svc-a", "private-a", "alice", "", "")
	denied, _ := store.Create("svc-b", "private-b", "bob", "", "")
	revoked, _ := store.Create("svc-c", "private-c", "carol", "", "")
	resolveAccessRequestForTest(t, store, approved.ID, accessrequests.Resolution{Status: accessrequests.StatusApproved, ResolvedBy: "admin"})
	resolveAccessRequestForTest(t, store, denied.ID, accessrequests.Resolution{Status: accessrequests.StatusDenied, ResolvedBy: "admin"})
	resolveAccessRequestForTest(t, store, revoked.ID, accessrequests.Resolution{Status: accessrequests.StatusRevoked, ResolvedBy: "admin"})

	h := NewHandler(cache.NewServiceCache(), nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				Groups:            []string{"admin"},
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "admin-subject"},
				PreferredUsername: "admin",
			}, true
		}),
	)

	tests := []struct {
		query  string
		wantID string
	}{
		{"approved", approved.ID},
		{"denied", denied.ID},
		{"revoked", revoked.ID},
	}
	for _, tt := range tests {
		rr := doGet(t, h.Routes(), "/api/v1/admin/access-requests?status="+tt.query)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%s: expected 200, got %d — body: %s", tt.query, rr.Code, rr.Body.String())
		}
		var resp struct {
			AccessRequests []*accessrequests.AccessRequest `json:"accessRequests"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.AccessRequests) != 1 || resp.AccessRequests[0].ID != tt.wantID {
			t.Fatalf("status=%s: got %+v, want only %s", tt.query, resp.AccessRequests, tt.wantID)
		}
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

func TestHandleAdminAccessRequestSub_ApproveSetsExpiryAndAudit(t *testing.T) {
	store := newARStore(t)
	ar, err := store.CreateWithOwner("svc-a", "private-a", "default/private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	ttl := time.Hour
	h := NewHandler(cache.NewServiceCache(), nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithAccessRequestApprovalTTL(ttl),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				Name:              "Admin User",
				Email:             "admin@example.com",
				Groups:            []string{"admin"},
				PreferredUsername: "admin",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "admin-subject"},
			}, true
		}),
	)

	before := time.Now().UTC().Add(ttl)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/access-requests/"+ar.ID+"/approve", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	after := time.Now().UTC().Add(ttl)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var updated accessrequests.AccessRequest
	if err := json.NewDecoder(rr.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status != accessrequests.StatusApproved {
		t.Fatalf("expected approved, got %q", updated.Status)
	}
	if updated.ResolvedBy != "admin-subject" {
		t.Fatalf("expected stable approver subject, got %q", updated.ResolvedBy)
	}
	if updated.ResolvedByName != "Admin User" || updated.ResolvedByEmail != "admin@example.com" {
		t.Fatalf("expected approver name/email audit fields, got %+v", updated)
	}
	if updated.TargetOwner != "default/private-a" {
		t.Fatalf("expected target owner audit field, got %q", updated.TargetOwner)
	}
	if updated.ExpiresAt == nil || updated.ExpiresAt.Before(before) || updated.ExpiresAt.After(after) {
		t.Fatalf("expected expiresAt around one hour from now, got %v", updated.ExpiresAt)
	}
}

func TestHandleAdminAccessRequestSub_DeleteRevokesEntitlement(t *testing.T) {
	sc := cache.NewServiceCache()
	addPrivateService(sc, "svc-a", "private-a", []string{"team-a"})

	store := newARStore(t)
	ar, err := store.Create("svc-a", "private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:    accessrequests.StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(sc, nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(r *http.Request) (*auth.Claims, bool) {
			if strings.Contains(r.URL.Path, "/admin/") {
				return &auth.Claims{
					Groups:           []string{"admin"},
					RegisteredClaims: jwt.RegisteredClaims{Subject: "admin-subject"},
				}, true
			}
			return &auth.Claims{
				PreferredUsername: "alice",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "user-subject"},
			}, true
		}),
	)

	rr := doGet(t, h.Routes(), "/api/v1/services")
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 1 {
		t.Fatalf("expected approved service before revoke, got %+v", resp.Services)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/access-requests/"+ar.ID, nil)
	rr = httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from delete/revoke, got %d — body: %s", rr.Code, rr.Body.String())
	}

	rr = doGet(t, h.Routes(), "/api/v1/services")
	resp = ServiceResponse{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 0 {
		t.Fatalf("expected revoked request to remove access, got %+v", resp.Services)
	}
}

func TestApprovedAccessRequestGrantsOnlyTargetService(t *testing.T) {
	sc := cache.NewServiceCache()
	addPrivateService(sc, "svc-a", "private-a", []string{"team-a"})
	addPrivateService(sc, "svc-b", "private-b", []string{"team-b"})

	store := newARStore(t)
	ar, err := store.Create("svc-a", "private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:     accessrequests.StatusApproved,
		ResolvedBy: "admin-subject",
		ExpiresAt:  &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(sc, nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				PreferredUsername: "alice",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "user-subject"},
			}, true
		}),
	)

	rr := doGet(t, h.Routes(), "/api/v1/services")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 1 || resp.Services[0].ID != "svc-a" {
		t.Fatalf("expected only approved service svc-a, got %+v", resp.Services)
	}

	rr = doGet(t, h.Routes(), "/api/v1/services/svc-b")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected svc-b to remain forbidden, got %d", rr.Code)
	}
}

func TestExpiredAccessRequestNoLongerGrantsService(t *testing.T) {
	sc := cache.NewServiceCache()
	addPrivateService(sc, "svc-a", "private-a", []string{"team-a"})

	store := newARStore(t)
	ar, err := store.Create("svc-a", "private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(-time.Second)
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:    accessrequests.StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(sc, nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				PreferredUsername: "alice",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "user-subject"},
			}, true
		}),
	)

	rr := doGet(t, h.Routes(), "/api/v1/services")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 0 {
		t.Fatalf("expected expired approval to remove access, got %+v", resp.Services)
	}
}

func TestDeniedAccessRequestNoLongerGrantsService(t *testing.T) {
	sc := cache.NewServiceCache()
	addPrivateService(sc, "svc-a", "private-a", []string{"team-a"})

	store := newARStore(t)
	ar, err := store.Create("svc-a", "private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:     accessrequests.StatusApproved,
		ResolvedBy: "admin-subject",
		ExpiresAt:  &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(sc, nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithClaimsExtractor(func(_ *http.Request) (*auth.Claims, bool) {
			return &auth.Claims{
				PreferredUsername: "alice",
				RegisteredClaims:  jwt.RegisteredClaims{Subject: "user-subject"},
			}, true
		}),
	)
	rr := doGet(t, h.Routes(), "/api/v1/services")
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 1 {
		t.Fatalf("expected approved service before denial, got %+v", resp.Services)
	}

	resolveAccessRequestForTest(t, store, ar.ID, accessrequests.Resolution{
		Status:     accessrequests.StatusDenied,
		ResolvedBy: "admin-subject",
	})
	rr = doGet(t, h.Routes(), "/api/v1/services")
	resp = ServiceResponse{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 0 {
		t.Fatalf("expected denied request to remove access, got %+v", resp.Services)
	}
}

func TestCanAccessService_ApprovedSnapshotGrantsOnlyMatchingService(t *testing.T) {
	store := newARStore(t)
	ar, err := store.Create("svc-a", "private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	resolveAccessRequestForTest(t, store, ar.ID, accessrequests.Resolution{
		Status:    accessrequests.StatusApproved,
		ExpiresAt: &expiresAt,
	})

	h := NewHandler(cache.NewServiceCache(), nil, true, nil, nil, WithAccessRequestStore(store))
	principal := wshub.Principal{
		Subject:       "user-subject",
		Authenticated: true,
	}

	if !h.CanAccessService(&cache.ServiceInfo{
		UID:            "svc-a",
		Visibility:     "private",
		RequiredGroups: []string{"team-a"},
	}, principal) {
		t.Fatal("expected approved service UID to grant access")
	}
	if h.CanAccessService(&cache.ServiceInfo{
		UID:            "svc-b",
		Visibility:     "private",
		RequiredGroups: []string{"team-b"},
	}, principal) {
		t.Fatal("approval for svc-a must not grant access to svc-b")
	}
}

func TestCanAccessService_RefreshesApprovedSnapshotAfterInterval(t *testing.T) {
	store := newARStore(t)
	ar, err := store.Create("svc-a", "private-a", "user-subject", "alice@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:    accessrequests.StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatal(err)
	}

	h := NewHandler(cache.NewServiceCache(), nil, true, nil, nil,
		WithAccessRequestStore(store),
		WithAccessRequestRefreshInterval(5*time.Millisecond),
	)
	principal := wshub.Principal{
		Subject:       "user-subject",
		Authenticated: true,
	}
	service := &cache.ServiceInfo{
		UID:            "svc-a",
		Visibility:     "private",
		RequiredGroups: []string{"team-a"},
	}
	if !h.CanAccessService(service, principal) {
		t.Fatal("expected approved service UID to grant access")
	}
	if _, err := store.Resolve(ar.ID, accessrequests.Resolution{
		Status:     accessrequests.StatusRevoked,
		ResolvedBy: "admin-subject",
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	if h.CanAccessService(service, principal) {
		t.Fatal("expected cached WebSocket approval to refresh after revoke")
	}
}
