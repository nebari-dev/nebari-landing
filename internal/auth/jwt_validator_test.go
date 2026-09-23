// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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

func jwkForKey(key *rsa.PrivateKey, kid string) map[string]interface{} {
	return map[string]interface{}{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"n":   encodeBase64URL(key.N),
		"e":   base64.RawURLEncoding.EncodeToString(eToBytes(key.E)),
	}
}

func jwksForKey(key *rsa.PrivateKey) map[string]interface{} {
	return map[string]interface{}{
		"keys": []map[string]interface{}{
			jwkForKey(key, testKID),
		},
	}
}

func startJWKSServer(t *testing.T, key *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	jwks := jwksForKey(key)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func signJWT(t *testing.T, key *rsa.PrivateKey, issuer string, exp time.Time, extra *Claims) string {
	t.Helper()
	return signJWTWithKID(t, key, issuer, exp, testKID, extra)
}

func signJWTWithKID(t *testing.T, key *rsa.PrivateKey, issuer string, exp time.Time, kid string, extra *Claims) string {
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
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

// newValidator returns a validator pointing at srv and blocks until its
// background JWKS fetch has succeeded. Stop() is registered on cleanup so
// the goroutine does not leak between tests.
func newValidator(t *testing.T, srv *httptest.Server) *JWTValidator {
	t.Helper()
	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)
	waitReady(t, v, 2*time.Second)
	return v
}

// waitReady polls v.Ready() until it returns true or the deadline expires.
// Tests use this in lieu of synchronous error returns from the constructor.
func waitReady(t *testing.T, v *JWTValidator, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v.Ready() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("validator never became ready within %v", timeout)
}

// waitForCalls polls calls until it reaches want or the deadline expires.
// Used by tests that need to observe an exact retry-budget count before
// asserting and stopping the validator. The counter is atomic because the
// HTTP handler goroutine and the test goroutine both touch it.
func waitForCalls(t *testing.T, calls *atomic.Int32, want int32, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if calls.Load() >= want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("expected at least %d calls within %v, got %d", want, timeout, calls.Load())
}

func waitForRefreshFailures(t *testing.T, v *JWTValidator, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if v.Stats().JWKSRefreshFailures > 0 {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("expected a JWKS refresh failure within %v", timeout)
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

// assertNotReady waits a brief window then asserts Ready() never flipped.
// All retries in tests use withNoBackoff (no sleeps), so the active retry
// budget completes in microseconds; a few ms is enough to observe failure.
func assertNotReady(t *testing.T, v *JWTValidator, settle time.Duration) {
	t.Helper()
	time.Sleep(settle)
	if v.Ready() {
		t.Error("validator unexpectedly became ready")
	}
}

func TestNewJWTValidator_InvalidURL_StaysNotReady(t *testing.T) {
	withNoBackoff(t, 1)
	v := NewJWTValidator("http://127.0.0.1:1", "realm")
	t.Cleanup(v.Stop)
	assertNotReady(t, v, 50*time.Millisecond)
}

func TestNewJWTValidator_EmptyKeys_StaysNotReady(t *testing.T) {
	withNoBackoff(t, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{}})
	}))
	defer srv.Close()
	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)
	assertNotReady(t, v, 50*time.Millisecond)
}

func TestNewJWTValidator_Success(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
	if !v.Ready() {
		t.Error("expected Ready() to be true after successful JWKS fetch")
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

func TestValidateToken_UnknownKIDRefreshesAreCoalescedAndCooledDown(t *testing.T) {
	withJWTRefreshSettings(t, 200*time.Millisecond, time.Minute, 1024)

	key := generateTestKey(t)
	jwks := jwksForKey(key)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n > 1 {
			time.Sleep(75 * time.Millisecond)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v := newValidator(t, srv)
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected initial JWKS fetch only, got %d calls", got)
	}

	v.keysMu.Lock()
	v.lastFetch = time.Now().Add(-2 * jwksStaleRefreshInterval)
	v.keysMu.Unlock()

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	const requests = 20
	tokens := make([]string, requests)
	for i := range tokens {
		tokens[i] = signJWTWithKID(t, key, issuer, time.Now().Add(time.Hour), fmt.Sprintf("unknown-%d", i), nil)
	}

	start := make(chan struct{})
	errCh := make(chan error, requests)
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			_, err := v.ValidateToken(token)
			errCh <- err
		}(tokens[i])
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err == nil {
			t.Error("expected unknown kid validation to fail")
		}
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected one coalesced request-time refresh, got %d JWKS calls", got)
	}

	stats := v.Stats()
	if stats.UnknownKIDTotal != requests {
		t.Errorf("unknown kid count: got %d, want %d", stats.UnknownKIDTotal, requests)
	}
	if stats.JWKSRefreshAttempts != uint64(calls.Load()) {
		t.Errorf("refresh attempts: got %d, want %d", stats.JWKSRefreshAttempts, calls.Load())
	}
	if stats.JWKSRefreshCoalesced == 0 {
		t.Error("expected concurrent refreshes to be coalesced")
	}
	if stats.UnknownKIDCacheEntries == 0 {
		t.Error("expected unknown kid negative cache entries")
	}
}

func TestValidateToken_ConcurrentRotatedKIDWaitsForSharedRefresh(t *testing.T) {
	withJWTRefreshSettings(t, 200*time.Millisecond, time.Minute, 1024)

	oldKey := generateTestKey(t)
	newKey := generateTestKey(t)
	const rotatedKID = "rotated-key"

	oldJWKS := jwksForKey(oldKey)
	rotatedJWKS := map[string]interface{}{
		"keys": []map[string]interface{}{
			jwkForKey(oldKey, testKID),
			jwkForKey(newKey, rotatedKID),
		},
	}

	var calls atomic.Int32
	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	var refreshOnce sync.Once
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_ = json.NewEncoder(w).Encode(oldJWKS)
			return
		}

		refreshOnce.Do(func() { close(refreshStarted) })
		<-releaseRefresh
		_ = json.NewEncoder(w).Encode(rotatedJWKS)
	}))
	defer srv.Close()

	v := newValidator(t, srv)
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected initial JWKS fetch only, got %d calls", got)
	}

	v.keysMu.Lock()
	v.lastFetch = time.Now().Add(-2 * jwksStaleRefreshInterval)
	v.keysMu.Unlock()

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	const requests = 8
	tokens := make([]string, requests)
	for i := range tokens {
		tokens[i] = signJWTWithKID(t, newKey, issuer, time.Now().Add(time.Hour), rotatedKID, &Claims{
			PreferredUsername: fmt.Sprintf("user-%d", i),
		})
	}

	start := make(chan struct{})
	type validationResult struct {
		claims *Claims
		err    error
	}
	resultCh := make(chan validationResult, requests)
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			claims, err := v.ValidateToken(token)
			resultCh <- validationResult{claims: claims, err: err}
		}(tokens[i])
	}
	close(start)

	select {
	case <-refreshStarted:
	case <-time.After(time.Second):
		t.Fatal("request-time JWKS refresh did not start")
	}
	close(releaseRefresh)

	wg.Wait()
	close(resultCh)

	for result := range resultCh {
		if result.err != nil {
			t.Errorf("rotated key should validate after shared refresh: %v", result.err)
			continue
		}
		if result.claims == nil || result.claims.PreferredUsername == "" {
			t.Errorf("expected claims from rotated token, got %#v", result.claims)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected one shared request-time refresh, got %d JWKS calls", got)
	}
	if stats := v.Stats(); stats.JWKSRefreshCoalesced == 0 {
		t.Error("expected concurrent rotated-key validations to share the refresh")
	}
}

func TestValidateToken_CooldownSkippedUnknownKIDDoesNotPoisonNegativeCache(t *testing.T) {
	withJWTRefreshSettings(t, 50*time.Millisecond, time.Minute, 1024)

	oldKey := generateTestKey(t)
	newKey := generateTestKey(t)
	const rotatedKID = "rotated-during-cooldown"

	oldJWKS := jwksForKey(oldKey)
	rotatedJWKS := map[string]interface{}{
		"keys": []map[string]interface{}{
			jwkForKey(oldKey, testKID),
			jwkForKey(newKey, rotatedKID),
		},
	}

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_ = json.NewEncoder(w).Encode(oldJWKS)
			return
		}
		_ = json.NewEncoder(w).Encode(rotatedJWKS)
	}))
	defer srv.Close()

	v := newValidator(t, srv)
	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	token := signJWTWithKID(t, newKey, issuer, time.Now().Add(time.Hour), rotatedKID, &Claims{PreferredUsername: "jdoe"})

	v.keysMu.Lock()
	v.lastFetch = time.Now()
	v.keysMu.Unlock()

	if _, err := v.ValidateToken(token); err == nil || !strings.Contains(err.Error(), "key refresh skipped by cooldown") {
		t.Fatalf("expected cooldown-skipped unknown kid error, got %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cooldown skip should not fetch JWKS, got %d calls", got)
	}
	if entries := v.Stats().UnknownKIDCacheEntries; entries != 0 {
		t.Fatalf("cooldown-skipped kid should not be negative-cached, got %d entries", entries)
	}

	time.Sleep(2 * jwksRefreshMinInterval)
	got, err := v.ValidateToken(token)
	if err != nil {
		t.Fatalf("rotated key should validate after cooldown refresh: %v", err)
	}
	if got.PreferredUsername != "jdoe" {
		t.Errorf("username: got %q, want jdoe", got.PreferredUsername)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("expected initial fetch plus post-cooldown refresh, got %d calls", got)
	}
}

func TestValidateToken_UnknownKIDNegativeCacheSkipsRefreshAfterCooldown(t *testing.T) {
	withJWTRefreshSettings(t, 10*time.Millisecond, time.Minute, 1024)

	key := generateTestKey(t)
	jwks := jwksForKey(key)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v := newValidator(t, srv)
	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	token := signJWTWithKID(t, key, issuer, time.Now().Add(time.Hour), "missing-key", nil)

	v.keysMu.Lock()
	v.lastFetch = time.Now().Add(-2 * jwksStaleRefreshInterval)
	v.keysMu.Unlock()
	if _, err := v.ValidateToken(token); err == nil {
		t.Fatal("expected first unknown kid validation to fail")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected initial fetch plus first refresh, got %d calls", got)
	}

	time.Sleep(2 * jwksRefreshMinInterval)
	if _, err := v.ValidateToken(token); err == nil {
		t.Fatal("expected cached unknown kid validation to fail")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("negative cache should skip second refresh, got %d JWKS calls", got)
	}
	if hits := v.Stats().UnknownKIDCacheHits; hits == 0 {
		t.Error("expected unknown kid cache hit to be observable")
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

func TestValidateToken_TooLargeTokenRejectedBeforeParsing(t *testing.T) {
	v := &JWTValidator{}
	v.ready.Store(true)

	_, err := v.ValidateToken(strings.Repeat("a", maxJWTBytes+1))
	if err == nil {
		t.Fatal("expected oversized token to fail")
	}
}

func TestValidateToken_ValidTokenSurvivesInvalidJWKSRefresh(t *testing.T) {
	key := generateTestKey(t)
	jwks := jwksForKey(key)
	var invalid atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if invalid.Load() {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{}})
			return
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v := newValidator(t, srv)
	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	tokenStr := signJWT(t, key, issuer, time.Now().Add(time.Hour), &Claims{PreferredUsername: "jdoe"})

	invalid.Store(true)
	v.keysMu.Lock()
	v.lastFetch = time.Now().Add(-2 * jwksStaleRefreshInterval)
	v.keysMu.Unlock()

	got, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("valid token should use last-known-good key after bad JWKS refresh: %v", err)
	}
	if got.PreferredUsername != "jdoe" {
		t.Errorf("username: got %q, want jdoe", got.PreferredUsername)
	}
	waitForRefreshFailures(t, v, time.Second)
}

func TestValidateToken_StaleCachedKeyDoesNotWaitForJWKSRefresh(t *testing.T) {
	key := generateTestKey(t)
	jwks := jwksForKey(key)
	var calls atomic.Int32
	var blockRefresh atomic.Bool
	releaseRefresh := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(releaseRefresh) })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if blockRefresh.Load() {
			<-releaseRefresh
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v := newValidator(t, srv)
	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	tokenStr := signJWT(t, key, issuer, time.Now().Add(time.Hour), &Claims{PreferredUsername: "jdoe"})

	blockRefresh.Store(true)
	v.keysMu.Lock()
	v.lastFetch = time.Now().Add(-2 * jwksStaleRefreshInterval)
	v.keysMu.Unlock()

	start := time.Now()
	got, err := v.ValidateToken(tokenStr)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("valid token should use cached key while stale refresh runs: %v", err)
	}
	if got.PreferredUsername != "jdoe" {
		t.Errorf("username: got %q, want jdoe", got.PreferredUsername)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("cached-key validation waited for JWKS refresh: %v", elapsed)
	}
	waitForCalls(t, &calls, 2, time.Second)
	releaseOnce.Do(func() { close(releaseRefresh) })
	waitForRefreshFailures(t, v, time.Second)
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

func withJWTRefreshSettings(t *testing.T, minInterval, negativeTTL time.Duration, negativeMaxEntries int) {
	t.Helper()
	origMinInterval := jwksRefreshMinInterval
	origNegativeTTL := unknownKIDCacheTTL
	origNegativeMaxEntries := unknownKIDCacheMaxEntries

	jwksRefreshMinInterval = minInterval
	unknownKIDCacheTTL = negativeTTL
	unknownKIDCacheMaxEntries = negativeMaxEntries

	t.Cleanup(func() {
		jwksRefreshMinInterval = origMinInterval
		unknownKIDCacheTTL = origNegativeTTL
		unknownKIDCacheMaxEntries = origNegativeMaxEntries
	})
}

func TestNewJWTValidator_404_StaysNotReady(t *testing.T) {
	// Regression: KEYCLOAK_URL pointing to a wrong endpoint returns HTTP 404.
	withNoBackoff(t, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)
	assertNotReady(t, v, 50*time.Millisecond)
}

func TestNewJWTValidator_RetriesOnTransientFailure(t *testing.T) {
	// First 2 requests fail with 503; the third succeeds — Ready() should flip.
	withNoBackoff(t, 5)

	key := generateTestKey(t)
	var calls atomic.Int32
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
		n := calls.Add(1)
		if n <= 2 {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)
	waitReady(t, v, 2*time.Second)
	if got := calls.Load(); got != 3 {
		t.Errorf("expected 3 calls to JWKS endpoint, got %d", got)
	}
}

func TestNewJWTValidator_ExhaustsRetries_StaysNotReady(t *testing.T) {
	// Server always fails — after maxRetries attempts Ready() must remain false
	// and the active retry budget must be honored (the goroutine then enters the
	// slow-poll loop, which uses real time.After and so will not fire before the
	// test calls Stop()).
	const attempts int32 = 3
	withNoBackoff(t, int(attempts))

	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)
	waitForCalls(t, &calls, attempts, time.Second)
	// give the goroutine a chance to enter slow-poll (which uses real
	// time.After(slowPollInterval) — 30 s in production, so no extra fetches
	// fire within this window).
	time.Sleep(50 * time.Millisecond)
	if v.Ready() {
		t.Error("validator unexpectedly became ready")
	}
	if got := calls.Load(); got != attempts {
		t.Errorf("expected exactly %d attempts, got %d", attempts, got)
	}
}

func TestNewJWTValidator_BackoffDoublesOnEachRetry(t *testing.T) {
	// Verify that the delay passed to retryDelay doubles on each attempt.
	withNoBackoff(t, 4)

	const initial = 100 * time.Millisecond
	retryInitialBackoff = initial

	var delays []time.Duration
	var delaysMu sync.Mutex
	retryDelay = func(d time.Duration) {
		delaysMu.Lock()
		delays = append(delays, d)
		delaysMu.Unlock()
	}

	// Server always fails so we exercise all retry gaps.
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "fail", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)
	waitForCalls(t, &calls, 4, time.Second)
	// Give the goroutine a beat to record the final inter-attempt delay.
	time.Sleep(20 * time.Millisecond)

	delaysMu.Lock()
	got := append([]time.Duration(nil), delays...)
	delaysMu.Unlock()

	// With 4 attempts there are 3 inter-attempt delays.
	if len(got) != 3 {
		t.Fatalf("expected 3 delays, got %d: %v", len(got), got)
	}
	expected := []time.Duration{initial, initial * 2, initial * 4}
	for i, d := range got {
		if d != expected[i] {
			t.Errorf("delay[%d]: got %v, want %v", i, d, expected[i])
		}
	}
}

func TestValidateToken_NotReady_ReturnsErrNotReady(t *testing.T) {
	// Server hangs so the goroutine is stuck on the first attempt and Ready()
	// stays false; ValidateToken must return ErrNotReady rather than a generic
	// validation failure so handlers can surface 503 instead of 401.
	withNoBackoff(t, 1)

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer func() { close(block); srv.Close() }()

	v := NewJWTValidator(srv.URL, testRealm)
	t.Cleanup(v.Stop)

	_, err := v.ValidateToken("any-token")
	if !errors.Is(err, ErrNotReady) {
		t.Errorf("expected ErrNotReady, got %v", err)
	}
}

// --- SetIssuerURL ---

func TestSetIssuerURL_SetsURL(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)
	v.SetIssuerURL("https://keycloak.example.com")
	if v.issuerURL != "https://keycloak.example.com" {
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

// --- missing optional claims ---

func TestValidateToken_MissingPreferredUsername_ReturnsEmptyString(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	claims := &Claims{Email: "user@example.com"}
	tokenStr := signJWT(t, key, issuer, time.Now().Add(time.Hour), claims)

	got, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken should succeed with missing preferred_username: %v", err)
	}
	if got.PreferredUsername != "" {
		t.Errorf("expected empty PreferredUsername, got %q", got.PreferredUsername)
	}
	if got.Email != "user@example.com" {
		t.Errorf("expected email preserved, got %q", got.Email)
	}
}

func TestValidateToken_MissingEmail_ReturnsEmptyString(t *testing.T) {
	key := generateTestKey(t)
	srv := startJWKSServer(t, key)
	v := newValidator(t, srv)

	issuer := fmt.Sprintf("%s/realms/%s", srv.URL, testRealm)
	claims := &Claims{PreferredUsername: "jdoe"}
	tokenStr := signJWT(t, key, issuer, time.Now().Add(time.Hour), claims)

	got, err := v.ValidateToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateToken should succeed with missing email: %v", err)
	}
	if got.Email != "" {
		t.Errorf("expected empty Email, got %q", got.Email)
	}
	if got.PreferredUsername != "jdoe" {
		t.Errorf("expected username preserved, got %q", got.PreferredUsername)
	}
}
