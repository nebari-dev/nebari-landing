// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
)

// Regression tests for issue #85 (decouple JWKS init from HTTP server start).
//
// The contract the rest of the system depends on:
//
//   - While the JWT validator's initial JWKS fetch has not yet completed
//     (Ready() == false), any handler that consumes a Bearer token must
//     return 503 Service Unavailable + Retry-After:5 instead of silently
//     degrading the request to anonymous (which hides private services, or
//     marks the user as "not logged in" in the SPA).
//   - Anonymous requests (no Authorization header) must continue to work as
//     usual — they don't depend on the validator and would otherwise be
//     gratuitously broken during the warmup window.
//
// If any of these assertions starts failing in the future, the most likely
// culprit is someone removing the writeAuthWarmupResponse path from one of
// the call sites or refactoring extractAndValidateJWT to drop the third
// (error) return.

// newWarmupValidator returns a *JWTValidator whose initial JWKS fetch will
// never complete (the test server blocks). Ready() therefore stays false for
// the lifetime of the test.
//
// Cleanup order matters: t.Cleanup is LIFO, and v.Stop() blocks on the
// in-flight fetchPublicKeys() returning. We register v.Stop *first* so the
// later-registered server cleanup runs *before* it — close(block) unblocks
// the hanging handler so the HTTP roundtrip returns immediately, instead of
// making v.Stop wait the full 10 s fetch timeout.
func newWarmupValidator(t *testing.T) *auth.JWTValidator {
	t.Helper()
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	v := auth.NewJWTValidator(srv.URL, "nebari")
	t.Cleanup(v.Stop)
	t.Cleanup(func() {
		close(block)
		srv.Close()
	})
	if v.Ready() {
		t.Fatal("validator unexpectedly became ready against a hanging server")
	}
	return v
}

func TestAuthWarmup_RequireAuthEndpoint_Returns503(t *testing.T) {
	// /api/v1/pins is a requireAuth-gated endpoint: with a Bearer header
	// while the validator is warming up, callers must receive 503, not 401.
	// We give it a real PinStore so requireAuth is actually reached (the
	// handler returns 501 if the store is nil — that short-circuit happens
	// before auth).
	v := newWarmupValidator(t)
	ps := newPinStore(t)
	h := NewHandler(cache.NewServiceCache(), v, true, nil, ps)

	rr := doGet(t, h.Routes(), "/api/v1/pins", "Authorization", "Bearer fake-token")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Retry-After"); got != "5" {
		t.Errorf("expected Retry-After: 5, got %q", got)
	}
}

func TestAuthWarmup_GetServicesWithBearer_Returns503(t *testing.T) {
	// /api/v1/services normally serves a degraded (public-only) response when
	// the caller is anonymous. If the caller *did* present credentials we
	// must not silently hide their private services — return 503 instead.
	v := newWarmupValidator(t)
	h := NewHandler(cache.NewServiceCache(), v, true, nil, nil)

	rr := doGet(t, h.Routes(), "/api/v1/services", "Authorization", "Bearer fake-token")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Retry-After"); got != "5" {
		t.Errorf("expected Retry-After: 5, got %q", got)
	}
}

func TestAuthWarmup_CallerIdentityWithBearer_Returns503(t *testing.T) {
	// The SPA polls /api/v1/caller-identity to render "logged in as X". If
	// the validator silently degraded to anonymous during warmup, the SPA
	// would flicker to a logged-out state. 503 keeps the prior auth state
	// stable while the SPA retries.
	v := newWarmupValidator(t)
	h := NewHandler(cache.NewServiceCache(), v, true, nil, nil)

	rr := doGet(t, h.Routes(), "/api/v1/caller-identity", "Authorization", "Bearer fake-token")

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Retry-After"); got != "5" {
		t.Errorf("expected Retry-After: 5, got %q", got)
	}
}

func TestAuthWarmup_AnonymousRequest_StillWorks(t *testing.T) {
	// Without an Authorization header, the request never touches the
	// validator — anonymous browsing must continue to work during warmup
	// (this is the whole reason we split the warmup check on the presence
	// of the header rather than blocking everything).
	v := newWarmupValidator(t)
	h := NewHandler(buildCache(entry{"u1", "pub", "public", "", 0}), v, true, nil, nil)

	rr := doGet(t, h.Routes(), "/api/v1/services")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for anonymous GET, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Retry-After"); got != "" {
		t.Errorf("anonymous request should not carry Retry-After, got %q", got)
	}

	rr = doGet(t, h.Routes(), "/api/v1/caller-identity")
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 for anonymous caller-identity, got %d", rr.Code)
	}
	var body CallerIdentityResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Authenticated {
		t.Error("expected authenticated:false for anonymous warmup request")
	}
}
