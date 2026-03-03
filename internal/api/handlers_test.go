// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	"github.com/nebari-dev/nebari-landing/internal/cache"
)

// --- helpers ---

func buildCache(entries ...struct {
	uid, name, visibility, category string
	priority                        int
}) *cache.ServiceCache {
	sc := cache.NewServiceCache()
	for _, e := range entries {
		sc.Add(&sdapp.App{
			UID:        e.uid,
			Name:       e.name,
			Namespace:  "default",
			Hostname:   e.name + ".example.com",
			TLSEnabled: true,
			LandingPage: &sdapp.LandingPage{
				Enabled:     true,
				DisplayName: e.name,
				Category:    e.category,
				Priority:    e.priority,
				Visibility:  e.visibility,
			},
		})
	}
	return sc
}

// newHandler returns a Handler with auth disabled.
func newTestHandler(sc *cache.ServiceCache) *Handler {
	return NewHandler(sc, nil, false, nil, nil)
}

// newAuthHandler returns a Handler with auth enabled but no validator.
func newAuthTestHandler(sc *cache.ServiceCache) *Handler {
	return NewHandler(sc, nil, true, nil, nil)
}

type entry = struct {
	uid, name, visibility, category string
	priority                        int
}

func doGet(t *testing.T, h http.Handler, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// --- /api/v1/health ---

func TestHandleHealth_ReturnsOK(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/health")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "healthy" {
		t.Errorf(`expected "healthy", got %v`, body)
	}
}

func TestHandleHealth_ContentTypeJSON(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/health")
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
}

// --- /api/v1/categories ---

func TestHandleGetCategories_ReturnsSortedUnique(t *testing.T) {
	sc := buildCache(
		entry{"u1", "a", "public", "Monitoring", 0},
		entry{"u2", "b", "public", "Development", 0},
		entry{"u3", "c", "public", "Monitoring", 0},
	)
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/categories")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string][]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	cats := body["categories"]
	want := []string{"Development", "Monitoring"}
	if len(cats) != len(want) {
		t.Fatalf("expected %v, got %v", want, cats)
	}
	for i, c := range cats {
		if c != want[i] {
			t.Errorf("pos %d: expected %q, got %q", i, want[i], c)
		}
	}
}

func TestHandleGetCategories_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- /api/v1/services ---

func TestHandleGetServices_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleGetServices_AuthDisabled_OnlyPublicVisible(t *testing.T) {
	sc := buildCache(
		entry{"u1", "pub", "public", "", 0},
		entry{"u2", "auth", "authenticated", "", 0},
		entry{"u3", "priv", "private", "", 0},
	)
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 1 {
		t.Errorf("expected 1 service (public only), got %d", len(resp.Services))
	}
	if len(resp.Services) > 0 && resp.Services[0].ID != "u1" {
		t.Errorf("expected public service 'u1', got %q", resp.Services[0].ID)
	}
}

func TestHandleGetServices_AuthEnabledNoToken_OnlyPublicVisible(t *testing.T) {
	sc := buildCache(
		entry{"u1", "pub", "public", "", 0},
		entry{"u2", "auth", "authenticated", "", 0},
	)
	rr := doGet(t, newAuthTestHandler(sc).Routes(), "/api/v1/services")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 1 {
		t.Errorf("expected 1 service without token, got %d", len(resp.Services))
	}
}

func TestHandleGetServices_DefaultVisibility_TreatedAsAuthenticated(t *testing.T) {
	sc := cache.NewServiceCache()
	sc.Add(&sdapp.App{
		UID:        "u-def",
		Name:       "default-vis",
		Namespace:  "ns",
		Hostname:   "h.com",
		TLSEnabled: true,
		LandingPage: &sdapp.LandingPage{
			Enabled:    true,
			Visibility: "", // no Visibility → defaults to "authenticated"
		},
	})
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services")
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	// auth disabled + default visibility → service is not shown to unauthenticated callers
	if len(resp.Services) != 0 {
		t.Errorf("expected 0 services without token, got %d", len(resp.Services))
	}
}

func TestHandleGetServices_ServiceView_CategoryIsArray(t *testing.T) {
	sc := buildCache(
		entry{"u1", "a", "public", "Tools", 0},
		entry{"u2", "b", "public", "Data", 0},
	)
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services")
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(resp.Services))
	}
	for _, svc := range resp.Services {
		if len(svc.Category) != 1 {
			t.Errorf("expected category array of length 1, got %v", svc.Category)
		}
	}
}

func TestHandleGetServices_ServiceView_HasRequiredFields(t *testing.T) {
	sc := buildCache(entry{"uid-1", "grafana", "public", "Monitoring", 0})
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services")
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(resp.Services))
	}
	svc := resp.Services[0]
	if svc.ID != "uid-1" {
		t.Errorf("expected id 'uid-1', got %q", svc.ID)
	}
	if svc.Status == "" {
		t.Error("expected non-empty status")
	}
	if svc.Pinned {
		t.Error("expected pinned=false without a pin store")
	}
}

// --- /api/v1/services/{id} ---

func TestHandleGetServiceByUID_NotFound(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/services/does-not-exist")
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetServiceByUID_PublicAccess_NoAuth(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services/uid-pub")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var svc ServiceView
	if err := json.NewDecoder(rr.Body).Decode(&svc); err != nil {
		t.Fatal(err)
	}
	if svc.ID != "uid-pub" {
		t.Errorf("expected id 'uid-pub', got %q", svc.ID)
	}
}

func TestHandleGetServiceByUID_AuthenticatedService_Forbidden_NoAuth(t *testing.T) {
	sc := buildCache(entry{"uid-auth", "auth-svc", "authenticated", "", 0})
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services/uid-auth")
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestHandleGetServiceByUID_PrivateService_Forbidden_NoAuth(t *testing.T) {
	sc := buildCache(entry{"uid-priv", "priv-svc", "private", "", 0})
	rr := doGet(t, newTestHandler(sc).Routes(), "/api/v1/services/uid-priv")
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestHandleGetServiceByUID_MethodNotAllowed(t *testing.T) {
	sc := buildCache(entry{"u1", "pub", "public", "", 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/u1", nil)
	rr := httptest.NewRecorder()
	newTestHandler(sc).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandleGetServiceByUID_EmptyID_BadRequest(t *testing.T) {
	// /api/v1/services/ with trailing slash but no ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// --- /api/v1/services/{id}/request_access ---

func TestHandleRequestAccess_NotImplemented(t *testing.T) {
	sc := buildCache(entry{"uid-pub", "pub", "public", "", 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/services/uid-pub/request_access", nil)
	rr := httptest.NewRecorder()
	newTestHandler(sc).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

// --- CORS ---

func TestCORSHeaders_PresentOnAllRequests(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/health")
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header")
	}
}

func TestCORSHeaders_OptionsReturns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/services", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for OPTIONS, got %d", rr.Code)
	}
}

// --- /api/v1/caller-identity ---

func TestHandleCallerIdentity_NoToken_Unauthenticated(t *testing.T) {
	rr := doGet(t, newAuthTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/caller-identity")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp CallerIdentityResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Authenticated {
		t.Errorf("expected authenticated=false without token, got %v", resp)
	}
	if resp.Username != "" {
		t.Errorf("expected empty username, got %q", resp.Username)
	}
}

func TestHandleCallerIdentity_AuthDisabled_Unauthenticated(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/caller-identity")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp CallerIdentityResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	// auth disabled → no JWT validated → authenticated=false
	if resp.Authenticated {
		t.Errorf("expected authenticated=false when auth is disabled, got %v", resp)
	}
}

func TestHandleCallerIdentity_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/caller-identity", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- Auth header extraction ---

func TestHandleGetServices_InvalidAuthHeader_TreatedAsUnauthenticated(t *testing.T) {
	sc := buildCache(
		entry{"u1", "pub", "public", "", 0},
		entry{"u2", "auth", "authenticated", "", 0},
	)
	rr := doGet(t, newAuthTestHandler(sc).Routes(), "/api/v1/services",
		"Authorization", "NotBearer sometoken",
	)
	var resp ServiceResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	// only the public service should be returned with a bad auth header
	if len(resp.Services) != 1 {
		t.Errorf("expected 1 service (public) with bad auth header, got %d", len(resp.Services))
	}
}

// --- /api/v1/notifications ---

func TestHandleGetNotifications_ReturnsEmptyList(t *testing.T) {
	rr := doGet(t, newTestHandler(cache.NewServiceCache()).Routes(), "/api/v1/notifications")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	notifs, ok := body["notifications"]
	if !ok {
		t.Fatal("expected 'notifications' key in response")
	}
	list, ok := notifs.([]interface{})
	if !ok || len(list) != 0 {
		t.Errorf("expected empty notifications array, got %v", notifs)
	}
}

func TestHandleGetNotifications_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications", nil)
	rr := httptest.NewRecorder()
	newTestHandler(cache.NewServiceCache()).Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
