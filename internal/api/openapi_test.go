// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-landing/internal/cache"
)

// docsEnabledHandler builds a Handler with the docs routes registered, no auth,
// no stores. Suitable for testing the docs surface in isolation.
func docsEnabledHandler() *Handler {
	return NewHandler(cache.NewServiceCache(), nil, false, nil, nil, WithDocsEnabled())
}

func TestDocs_DisabledByDefault_Returns404(t *testing.T) {
	h := NewHandler(cache.NewServiceCache(), nil, false, nil, nil)
	for _, path := range []string{"/api/v1/docs", "/api/v1/docs/openapi.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		h.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("%s: expected 404 when docs are disabled, got %d", path, rr.Code)
		}
	}
}

func TestDocs_OpenAPISpec_ReturnsValidJSON(t *testing.T) {
	h := docsEnabledHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs/openapi.json", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %q", ct)
	}

	var spec map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &spec); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if v, _ := spec["openapi"].(string); !strings.HasPrefix(v, "3.") {
		t.Errorf("expected openapi version 3.x, got %q", v)
	}
	if _, ok := spec["paths"].(map[string]any); !ok {
		t.Error("spec missing 'paths' object")
	}
}

func TestDocs_Viewer_ReturnsHTML(t *testing.T) {
	h := docsEnabledHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/docs", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", rr.Header().Get("Content-Type"))
	}
	body := rr.Body.String()
	// The template uses a relative <a href="openapi.json"> so it resolves to
	// /api/v1/docs/openapi.json when the browser is at /api/v1/docs, regardless
	// of any future base-path change. Substring match covers both forms.
	if !strings.Contains(body, "openapi.json") {
		t.Error("viewer HTML should reference the spec endpoint")
	}
}

func TestDocs_NonGet_MethodNotAllowed(t *testing.T) {
	h := docsEnabledHandler()
	for _, path := range []string{"/api/v1/docs", "/api/v1/docs/openapi.json"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		rr := httptest.NewRecorder()
		h.Routes().ServeHTTP(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s POST: expected 405, got %d", path, rr.Code)
		}
	}
}
