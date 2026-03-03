// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	sdapp "github.com/nebari-dev/nebari-landing/nebari-webapi/internal/app"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/cache"
	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/pins"
)

// newPinStore creates a temp-dir-backed PinStore for tests.
func newPinStore(t *testing.T) *pins.PinStore {
	t.Helper()
	ps, err := pins.NewPinStore(filepath.Join(t.TempDir(), "pins.db"))
	if err != nil {
		t.Fatalf("NewPinStore: %v", err)
	}
	t.Cleanup(func() { _ = ps.Close() })
	return ps
}

// newPinHandler builds a Handler with auth disabled and a real PinStore.
func newPinHandler(t *testing.T, sc *cache.ServiceCache) (*Handler, *pins.PinStore) {
	t.Helper()
	ps := newPinStore(t)
	h := NewHandler(sc, nil, false, nil, ps)
	return h, ps
}

func doMethod(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func addApp(sc *cache.ServiceCache, uid, name string) {
	sc.Add(&sdapp.App{
		UID:        uid,
		Name:       name,
		Namespace:  "default",
		Hostname:   name + ".example.com",
		TLSEnabled: true,
		LandingPage: &sdapp.LandingPage{
			Enabled:     true,
			DisplayName: name,
			Visibility:  "public",
			Priority:    10,
		},
	})
}

// --- GET /api/v1/pins ---

func TestHandleGetPins_NoPinStore_Returns501(t *testing.T) {
	h := newTestHandler(cache.NewServiceCache()) // pinStore=nil
	rr := doGet(t, h.Routes(), "/api/v1/pins")
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandleGetPins_EmptyForNewUser(t *testing.T) {
	sc := cache.NewServiceCache()
	h, _ := newPinHandler(t, sc)
	rr := doGet(t, h.Routes(), "/api/v1/pins")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp PinsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Pins) != 0 {
		t.Errorf("expected empty pins, got %v", resp.Pins)
	}
	if len(resp.UIDs) != 0 {
		t.Errorf("expected empty UIDs, got %v", resp.UIDs)
	}
}

func TestHandleGetPins_ReturnsCachedService(t *testing.T) {
	sc := cache.NewServiceCache()
	addApp(sc, "uid-1", "grafana")
	h, ps := newPinHandler(t, sc)
	if err := ps.Pin("_anonymous", "uid-1"); err != nil {
		t.Fatalf("pre-pin: %v", err)
	}
	rr := doGet(t, h.Routes(), "/api/v1/pins")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp PinsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Pins) != 1 || resp.Pins[0].Name != "grafana" {
		t.Errorf("expected [grafana], got %+v", resp.Pins)
	}
	if len(resp.UIDs) != 1 || resp.UIDs[0] != "uid-1" {
		t.Errorf("expected UIDs=[uid-1], got %v", resp.UIDs)
	}
}

func TestHandleGetPins_StaleUID_AbsentFromPinsButInUIDs(t *testing.T) {
	sc := cache.NewServiceCache()
	h, ps := newPinHandler(t, sc)
	// Pin a UID that is not in the cache (deleted NebariApp)
	if err := ps.Pin("_anonymous", "stale-uid"); err != nil {
		t.Fatalf("pre-pin: %v", err)
	}
	rr := doGet(t, h.Routes(), "/api/v1/pins")
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp PinsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Pins) != 0 {
		t.Errorf("stale uid should not appear in Pins, got %v", resp.Pins)
	}
	if len(resp.UIDs) != 1 || resp.UIDs[0] != "stale-uid" {
		t.Errorf("stale uid should appear in UIDs, got %v", resp.UIDs)
	}
}

// --- PUT /api/v1/pins/{uid} ---

func TestHandlePinByUID_NoPinStore_Returns501(t *testing.T) {
	h := newTestHandler(cache.NewServiceCache())
	rr := doMethod(t, h.Routes(), http.MethodPut, "/api/v1/pins/uid-1")
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rr.Code)
	}
}

func TestHandlePinByUID_Put_PinsService(t *testing.T) {
	sc := cache.NewServiceCache()
	addApp(sc, "uid-1", "jupyter")
	h, ps := newPinHandler(t, sc)

	rr := doMethod(t, h.Routes(), http.MethodPut, "/api/v1/pins/uid-1")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	uids, _ := ps.Get("_anonymous")
	if len(uids) != 1 || uids[0] != "uid-1" {
		t.Errorf("expected [uid-1] in store, got %v", uids)
	}
}

func TestHandlePinByUID_Put_MissingUID_Returns400(t *testing.T) {
	h, _ := newPinHandler(t, cache.NewServiceCache())
	rr := doMethod(t, h.Routes(), http.MethodPut, "/api/v1/pins/")
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandlePinByUID_Put_Idempotent(t *testing.T) {
	h, ps := newPinHandler(t, cache.NewServiceCache())
	doMethod(t, h.Routes(), http.MethodPut, "/api/v1/pins/uid-1")
	doMethod(t, h.Routes(), http.MethodPut, "/api/v1/pins/uid-1")
	uids, _ := ps.Get("_anonymous")
	if len(uids) != 1 {
		t.Errorf("expected 1 pin (idempotent), got %d", len(uids))
	}
}

// --- DELETE /api/v1/pins/{uid} ---

func TestHandlePinByUID_Delete_UnpinsService(t *testing.T) {
	h, ps := newPinHandler(t, cache.NewServiceCache())
	_ = ps.Pin("_anonymous", "uid-1")

	rr := doMethod(t, h.Routes(), http.MethodDelete, "/api/v1/pins/uid-1")
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	uids, _ := ps.Get("_anonymous")
	if len(uids) != 0 {
		t.Errorf("expected empty after delete, got %v", uids)
	}
}

func TestHandlePinByUID_Delete_Idempotent(t *testing.T) {
	h, _ := newPinHandler(t, cache.NewServiceCache())
	rr := doMethod(t, h.Routes(), http.MethodDelete, "/api/v1/pins/uid-1")
	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204 for unknown uid, got %d", rr.Code)
	}
}

func TestHandlePinByUID_MethodNotAllowed(t *testing.T) {
	h, _ := newPinHandler(t, cache.NewServiceCache())
	rr := doMethod(t, h.Routes(), http.MethodPost, "/api/v1/pins/uid-1")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

// --- CORS ---

func TestCORS_PinEndpoint_AllowsDeleteAndPut(t *testing.T) {
	h, _ := newPinHandler(t, cache.NewServiceCache())
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/pins/uid-1", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	allowed := rr.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"PUT", "DELETE"} {
		if !containsMethod(allowed, method) {
			t.Errorf("CORS Allow-Methods %q missing %s", allowed, method)
		}
	}
}

func containsMethod(header, method string) bool {
	for _, part := range splitComma(header) {
		if part == method {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var out []string
	for _, p := range []byte(s) {
		_ = p
	}
	// simple split on ", "
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			part := trimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	if part := trimSpace(s[start:]); part != "" {
		out = append(out, part)
	}
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
