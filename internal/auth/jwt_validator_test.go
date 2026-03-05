// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package auth

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
)

// --- fixtures ---

const testKID = "test-key-id"
const testRealm = "nebari"

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func encodeBase64URL(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

func eToBytes(e int) []byte {
	b := [4]byte{byte(e >> 24), byte(e >> 16), byte(e >> 8), byte(e)}
	i := 0
	for i < 3 && b[i] == 0 {
		i++
	}
	return b[i:]
}

func startJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": testKID,
				"use": "sig",
				"n":   encodeBase64URL(key.N),
				"e":   base64.RawURLEncoding.EncodeToString(eToBytes(key.E)),
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func signJWT(t *testing.T, key *rsa.PrivateKey, issuer string, exp time.Time, extra *Claims) string {
	t.Helper()
	if extra == nil {
		extra = &Claims{}
	}
	extra.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(exp),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, extra)
	tok.Header["kid"] = testKID
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func newValidator(t *testing.T, srv *httptest.Server) *JWTValidator {
	t.Helper()
	v, err := NewJWTValidator(srv.URL, testRealm)
	if err != nil {
		t.Fatalf("NewJWTValidator: %v", err)
	}
	return v
}

// --- parseRSAPublicKey ---

func TestParseRSAPublicKey_ValidJWK(t *testing.T) {
	key := generateTestKey(t)
	jwk := JWK{
		Kty: "RSA",
		Kid: testKID,
		N:   encodeBase64URL(key.N),
		E:   base64.RawURLEncoding.EncodeToString(eToBytes(key.E)),
	}
	pub, err := parseRSAPublicKey(jwk)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pub.N.Cmp(key.N) != 0 {
		t.Error("N mismatch")
	}
	if pub.E != key.E {
		t.Errorf("E: got %d, want %d", pub.E, key.E)
	}
}

func TestParseRSAPublicKey_InvalidN(t *testing.T) {
	_, err := parseRSAPublicKey(JWK{Kty: "RSA", Kid: "k", N: "!!!not base64", E: "AQAB"})
	if err == nil {
		t.Error("expected error for invalid N")
	}
}

func TestParseRSAPublicKey_InvalidE(t *testing.T) {
	key := generateTestKey(t)
	_, err := parseRSAPublicKey(JWK{Kty: "RSA", Kid: "k", N: encodeBase64URL(key.N), E: "!!!"})
	if err == nil {
		t.Error("expected error for invalid E")
	}
}

// --- NewJWTValidator ---

func TestNewJWTValidator_InvalidURL_ReturnsError(t *testing.T) {
	withNoBackoff(t, 1)
	_, err := NewJWTValidator("http://127.0.0.1:1", "realm")
	if err == nil {
		t.Error("expected error connecting to invalid URL")
	}
}

func TestNewJWTValidator_EmptyKeys_ReturnsError(t *testing.T) {
	withNoBackoff(t, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{}})
	}))
	defer srv.Close()
	_, err := NewJWTValidator(srv.URL, testRealm)
	if err == nil {
		t.Error("expected error when JWKS has no keys")
	}
}

func TestNewJWTValidator_Success(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

// --- ValidateToken ---

func TestValidateToken_Valid(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	claims := &Claims{
		PreferredUsername: "jdoe",
		Email:             "jdoe@example.com",
		Name:              "John Doe",
		Groups:            []string{"admin", "data-science"},
	}
	tokenStr := signJWT(t, key, issuer, time.Now().Add(time.Hour), claims)

	got, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PreferredUsername != "jdoe" {
		t.Errorf("username: got %q, want jdoe", got.PreferredUsername)
	}
	if got.Email != "jdoe@example.com" {
		t.Errorf("email: got %q, want jdoe@example.com", got.Email)
	}
	if len(got.Groups) != 2 {
		t.Errorf("groups: got %v, want 2 items", got.Groups)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	tokenStr := signJWT(t, key, issuer, time.Now().Add(-time.Hour), nil)

	_, err := v.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateToken_WrongIssuer(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	tokenStr := signJWT(t, key, "https://wrong-issuer.example.com/realms/other", time.Now().Add(time.Hour), nil)

	_, err := v.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for wrong issuer")
	}
}

func TestValidateToken_UnknownKID(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	claims := &Claims{}
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "unknown-kid"
	tokenStr, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for unknown kid")
	}
}

func TestValidateToken_Tampered(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	tokenStr := signJWT(t, key, issuer, time.Now().Add(time.Hour), nil)

	tampered := tokenStr[:len(tokenStr)-5] + "xxxxx"
	_, err := v.ValidateToken(tampered)
	if err == nil {
		t.Error("expected error for tampered token")
	}
}

func TestValidateToken_WrongAlgorithm(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	claims := &Claims{}
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Issuer:    issuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = testKID
	tokenStr, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for wrong algorithm")
	}
}

// --- Claims struct ---

func TestClaims_JSON_RoundTrip(t *testing.T) {
	c := Claims{
		Email:             "user@example.com",
		Name:              "Test User",
		PreferredUsername: "testuser",
		Groups:            []string{"group-a", "group-b"},
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var got Claims
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Email != c.Email || got.PreferredUsername != c.PreferredUsername {
		t.Errorf("round-trip mismatch: got %+v, want %+v", got, c)
	}
	if len(got.Groups) != 2 {
		t.Errorf("expected 2 groups, got %v", got.Groups)
	}
}

// --- NewJWTValidator retry behaviour ---

// withNoBackoff overrides the retry knobs for the duration of a test so that
// retry loops complete instantly without sleeping.  It also caps the number of
// attempts to maxAttempts so tests stay deterministic.
func withNoBackoff(t *testing.T, maxAttempts int) {
	t.Helper()
	origDelay := retryDelay
	origMax := retryMaxAttempts
	origBackoff := retryInitialBackoff

	retryDelay = func(time.Duration) {} // no-op
	retryMaxAttempts = maxAttempts
	retryInitialBackoff = 0

	t.Cleanup(func() {
		retryDelay = origDelay
		retryMaxAttempts = origMax
		retryInitialBackoff = origBackoff
	})
}

func TestNewJWTValidator_404_ReturnsError(t *testing.T) {
	// Regression: KEYCLOAK_URL missing /auth context path causes HTTP 404.
	withNoBackoff(t, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := NewJWTValidator(srv.URL, testRealm)
	if err == nil {
		t.Fatal("expected error when server returns 404")
	}
}

func TestNewJWTValidator_RetriesOnTransientFailure(t *testing.T) {
	// First 2 requests fail with 503; the third succeeds — validator should be created.
	withNoBackoff(t, 5)

	key := generateTestKey(t)
	calls := 0
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": testKID,
				"use": "sig",
				"n":   encodeBase64URL(key.N),
				"e":   base64.RawURLEncoding.EncodeToString(eToBytes(key.E)),
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v, err := NewJWTValidator(srv.URL, testRealm)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls to JWKS endpoint, got %d", calls)
	}
}

func TestNewJWTValidator_ExhaustsRetries_ReturnsError(t *testing.T) {
	// Server always fails — after maxRetries attempts an error must be returned.
	const attempts = 3
	withNoBackoff(t, attempts)

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewJWTValidator(srv.URL, testRealm)
	if err == nil {
		t.Fatal("expected error when all retries are exhausted")
	}
	if calls != attempts {
		t.Errorf("expected exactly %d attempts, got %d", attempts, calls)
	}
}

func TestNewJWTValidator_BackoffDoublesOnEachRetry(t *testing.T) {
	// Verify that the delay passed to retryDelay doubles on each attempt.
	withNoBackoff(t, 4)

	const initial = 100 * time.Millisecond
	retryInitialBackoff = initial

	var delays []time.Duration
	retryDelay = func(d time.Duration) { delays = append(delays, d) }

	// Server always fails so we exercise all retry gaps.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, _ = NewJWTValidator(srv.URL, testRealm)

	// With 4 attempts there are 3 inter-attempt delays.
	if len(delays) != 3 {
		t.Fatalf("expected 3 delays, got %d: %v", len(delays), delays)
	}
	expected := []time.Duration{initial, initial * 2, initial * 4}
	for i, d := range delays {
		if d != expected[i] {
			t.Errorf("delay[%d]: got %v, want %v", i, d, expected[i])
		}
	}
}

// --- SetIssuerURL ---

func TestSetIssuerURL_SetsURL(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)
	v.SetIssuerURL("https://keycloak.example.com/auth")
	if v.issuerURL != "https://keycloak.example.com/auth" {
		t.Errorf("expected issuerURL to be set, got %q", v.issuerURL)
	}
}

func TestSetIssuerURL_EmptyString_IsNoOp(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)
	original := v.issuerURL
	v.SetIssuerURL("")
	if v.issuerURL != original {
		t.Errorf("empty SetIssuerURL should be no-op, issuerURL changed to %q", v.issuerURL)
	}
}

func TestSetIssuerURL_TrailingSlashStripped(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)
	v.SetIssuerURL("https://keycloak.example.com/")
	if v.issuerURL != "https://keycloak.example.com" {
		t.Errorf("expected trailing slash stripped, got %q", v.issuerURL)
	}
}
