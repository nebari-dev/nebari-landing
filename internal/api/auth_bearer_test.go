// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
)

const apiAuthTestKID = "api-auth-test-key"
const apiAuthTestRealm = "nebari"
const apiAuthExpectedAudience = "nebari-landingpage"
const apiAuthExpectedAuthorizedParty = "nebari-frontend-spa"

func TestBearerBindingFailureRejectsOrdinaryEndpoint(t *testing.T) {
	validator, key, issuer := newAPIAuthValidator(t)
	token := signAPIAuthToken(t, key, issuer, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{"other-api"},
		},
		PreferredUsername: "alice",
		TokenType:         "Bearer",
		AuthorizedParty:   apiAuthExpectedAuthorizedParty,
	})

	h := NewHandler(cache.NewServiceCache(), validator, true, nil, newPinStore(t))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pins", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected ordinary endpoint to reject wrong-audience token with 401, got %d", rr.Code)
	}
}

func TestBearerBindingFailureRejectsAdminEndpoint(t *testing.T) {
	validator, key, issuer := newAPIAuthValidator(t)
	token := signAPIAuthToken(t, key, issuer, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{apiAuthExpectedAudience},
		},
		PreferredUsername: "admin",
		Groups:            []string{"admin"},
		TokenType:         "Bearer",
		AuthorizedParty:   "other-client",
	})

	h := NewHandler(cache.NewServiceCache(), validator, true, nil, nil, WithAccessRequestStore(newARStore(t)))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/access-requests", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected admin endpoint to reject wrong-azp token with 401, got %d", rr.Code)
	}
}

func TestBearerBindingFailureRejectsServicesEndpoint(t *testing.T) {
	validator, key, issuer := newAPIAuthValidator(t)
	token := signAPIAuthToken(t, key, issuer, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{"other-api"},
		},
		PreferredUsername: "alice",
		TokenType:         "Bearer",
		AuthorizedParty:   apiAuthExpectedAuthorizedParty,
	})

	h := NewHandler(cache.NewServiceCache(), validator, true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected services endpoint to reject wrong-audience token with 401, got %d", rr.Code)
	}
}

func TestBearerBindingFailureRejectsServiceByIDEndpoint(t *testing.T) {
	validator, key, issuer := newAPIAuthValidator(t)
	token := signAPIAuthToken(t, key, issuer, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{"other-api"},
		},
		PreferredUsername: "alice",
		TokenType:         "Bearer",
		AuthorizedParty:   apiAuthExpectedAuthorizedParty,
	})

	h := NewHandler(buildCache(entry{"svc-1", "public-svc", "public", "", 0}), validator, true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/svc-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected service-by-id endpoint to reject wrong-audience token with 401, got %d", rr.Code)
	}
}

func TestBearerBindingFailureRejectsMissingServiceByIDEndpoint(t *testing.T) {
	validator, key, issuer := newAPIAuthValidator(t)
	token := signAPIAuthToken(t, key, issuer, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{"other-api"},
		},
		PreferredUsername: "alice",
		TokenType:         "Bearer",
		AuthorizedParty:   apiAuthExpectedAuthorizedParty,
	})

	h := NewHandler(cache.NewServiceCache(), validator, true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/missing", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing service lookup to reject wrong-audience token with 401, got %d", rr.Code)
	}
}

func TestBearerBindingFailureRejectsCallerIdentityEndpoint(t *testing.T) {
	validator, key, issuer := newAPIAuthValidator(t)
	token := signAPIAuthToken(t, key, issuer, &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience: jwt.ClaimStrings{apiAuthExpectedAudience},
		},
		PreferredUsername: "alice",
		TokenType:         "Bearer",
		AuthorizedParty:   "other-client",
	})

	h := NewHandler(cache.NewServiceCache(), validator, true, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/caller-identity", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.Routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected caller-identity endpoint to reject wrong-azp token with 401, got %d", rr.Code)
	}
}

func newAPIAuthValidator(t *testing.T) (*auth.JWTValidator, *rsa.PrivateKey, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": apiAuthTestKID,
				"use": "sig",
				"n":   encodeAPIAuthBase64URL(key.N),
				"e":   base64.RawURLEncoding.EncodeToString(apiAuthEToBytes(key.E)),
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)

	validator := auth.NewJWTValidator(srv.URL, apiAuthTestRealm)
	validator.SetExpectedTokenBinding(apiAuthExpectedAudience, apiAuthExpectedAuthorizedParty)
	t.Cleanup(validator.Stop)
	waitAPIAuthReady(t, validator, 2*time.Second)

	return validator, key, fmt.Sprintf("%s/realms/%s", srv.URL, apiAuthTestRealm)
}

func waitAPIAuthReady(t *testing.T, validator *auth.JWTValidator, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if validator.Ready() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("validator never became ready within %v", timeout)
}

func signAPIAuthToken(t *testing.T, key *rsa.PrivateKey, issuer string, claims *auth.Claims) string {
	t.Helper()
	registered := claims.RegisteredClaims
	registered.Issuer = issuer
	registered.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
	registered.IssuedAt = jwt.NewNumericDate(time.Now())
	claims.RegisteredClaims = registered

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = apiAuthTestKID
	token, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return token
}

func encodeAPIAuthBase64URL(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

func apiAuthEToBytes(e int) []byte {
	b := [4]byte{byte(e >> 24), byte(e >> 16), byte(e >> 8), byte(e)}
	i := 0
	for i < 3 && b[i] == 0 {
		i++
	}
	return b[i:]
}
