package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/sync/singleflight"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("jwt-validator")

// retryMaxAttempts and retryInitialBackoff control the *active* JWKS fetch
// retry loop run on the background init goroutine. After this budget is
// exhausted the goroutine switches to slowPollInterval to keep trying
// indefinitely without the exponential blow-up. They are package-level
// variables so tests can override them without incurring real sleep time.
var (
	retryMaxAttempts    = 5
	retryInitialBackoff = 2 * time.Second
	// retryDelay is called between attempts; replaced in tests to avoid sleeping.
	retryDelay = time.Sleep
	// slowPollInterval is the cadence for the post-retry "keep trying" loop.
	// 30 s is short enough to converge quickly when Keycloak finishes coming up
	// but long enough that an operator looking at logs sees a clear "still
	// failing" cadence rather than a tight retry storm.
	slowPollInterval = 30 * time.Second
	// jwksStaleRefreshInterval is the normal background refresh cadence driven
	// by request traffic after the validator has become ready.
	jwksStaleRefreshInterval = time.Hour
	// jwksRefreshMinInterval is a floor across on-demand refresh attempts. It
	// prevents unknown-kid traffic from mapping 1:1 to outbound JWKS requests.
	jwksRefreshMinInterval = time.Minute
	// unknownKIDCacheTTL remembers key IDs that remained unknown after refresh.
	unknownKIDCacheTTL = 5 * time.Minute
	// unknownKIDCacheMaxEntries bounds memory used by the negative cache.
	unknownKIDCacheMaxEntries = 1024
	// maxJWTBytes is an application-level cap before jwt.ParseWithClaims sees
	// untrusted input. Keycloak tokens are usually far smaller; 64 KiB leaves
	// room for large group claims while bounding pathological headers.
	maxJWTBytes = 64 * 1024
	// maxKIDBytes bounds the untrusted JWT header value we use as a map key.
	maxKIDBytes = 1024
)

// ErrNotReady is returned by ValidateToken when the validator's initial JWKS
// fetch has not completed yet. Callers should treat this as transient and
// surface a 503 Service Unavailable so clients distinguish "Keycloak not
// reachable yet" from "your token is bad" (401).
var ErrNotReady = errors.New("jwt validator: initial JWKS fetch not yet complete")

var errJWKSRefreshSkipped = errors.New("jwt validator: JWKS refresh skipped by cooldown")

// Claims represents the JWT claims we care about
type Claims struct {
	jwt.RegisteredClaims
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

// JWTValidator validates JWT tokens from Keycloak.
//
// The initial JWKS fetch runs asynchronously in a background goroutine started
// by NewJWTValidator so that process startup does not block on Keycloak being
// reachable. This matters in two scenarios:
//
//  1. Pod startup races: kubelet's liveness probe kills the webapi pod if it
//     does not bind to :8080 quickly enough. A synchronous retry loop here
//     blocks the HTTP server from starting and produces CrashLoopBackOff when
//     Keycloak is slow to become reachable.
//  2. JWKS rotation failures: if a later refresh fails, the validator keeps
//     serving with its existing keys rather than crashing the pod.
//
// While `ready` is false, ValidateToken returns ErrNotReady, which handlers
// surface as 503 Service Unavailable to distinguish "auth not online yet"
// from "your token is bad" (401).
type JWTValidator struct {
	keycloakURL string
	// issuerURL is used to validate the `iss` claim. It defaults to keycloakURL
	// but can be overridden via SetIssuerURL when the external Keycloak URL
	// (used in token `iss`) differs from the internal cluster URL used for
	// JWK fetching.
	issuerURL  string
	realm      string
	publicKeys map[string]*rsa.PublicKey
	keysMu     sync.RWMutex
	lastFetch  time.Time
	// refreshGroup coalesces concurrent request-time refreshes. The initial
	// background fetch path stays direct so startup retry logging remains simple.
	refreshGroup singleflight.Group
	refreshMu    sync.Mutex
	// lastRefreshAttempt tracks failed attempts too; lastFetch only records
	// successful key loads.
	lastRefreshAttempt time.Time
	refreshCall        *jwksRefreshCall
	unknownKIDMu       sync.Mutex
	unknownKIDs        map[string]time.Time
	// ready flips to true once the first JWKS fetch succeeds. Reads must use
	// atomic.Bool because the writer runs on the background init goroutine
	// while readers run on every request-handling goroutine.
	ready atomic.Bool
	// Counters are intentionally process-local; they surface via Stats() and the
	// existing debug endpoint so operators can see refresh pressure without a
	// separate metrics subsystem.
	jwksRefreshAttempts  atomic.Uint64
	jwksRefreshSuccesses atomic.Uint64
	jwksRefreshFailures  atomic.Uint64
	jwksRefreshSkipped   atomic.Uint64
	jwksRefreshCoalesced atomic.Uint64
	unknownKIDTotal      atomic.Uint64
	unknownKIDCacheHits  atomic.Uint64
	// stopCh is closed by Stop() to interrupt the slow-poll loop. The active
	// retry path uses retryDelay (overrideable in tests) and does not need
	// the channel — it always completes within retryMaxAttempts iterations.
	stopCh chan struct{}
	// doneCh closes when initLoop exits (either on success, on stop, or
	// before entering slow poll). Used by Stop() to wait for the goroutine.
	doneCh chan struct{}
	// stopOnce guards the close(stopCh) in Stop so concurrent callers cannot
	// double-close the channel.
	stopOnce sync.Once
}

type jwksRefreshCall struct {
	done chan struct{}
	err  error
}

// ValidatorStats exposes process-local counters for JWKS refresh behavior.
type ValidatorStats struct {
	Ready                  bool      `json:"ready"`
	KeyCount               int       `json:"key_count"`
	LastFetch              time.Time `json:"last_fetch,omitempty"`
	LastRefreshAttempt     time.Time `json:"last_refresh_attempt,omitempty"`
	JWKSRefreshAttempts    uint64    `json:"jwks_refresh_attempts"`
	JWKSRefreshSuccesses   uint64    `json:"jwks_refresh_successes"`
	JWKSRefreshFailures    uint64    `json:"jwks_refresh_failures"`
	JWKSRefreshSkipped     uint64    `json:"jwks_refresh_skipped"`
	JWKSRefreshCoalesced   uint64    `json:"jwks_refresh_coalesced"`
	UnknownKIDTotal        uint64    `json:"unknown_kid_total"`
	UnknownKIDCacheHits    uint64    `json:"unknown_kid_cache_hits"`
	UnknownKIDCacheEntries int       `json:"unknown_kid_cache_entries"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// NewJWTValidator creates a new JWT validator and returns immediately. The
// initial JWKS fetch runs on a background goroutine so the caller (typically
// process startup wiring up the HTTP server) does not block on Keycloak being
// reachable. See JWTValidator's doc comment for why this matters.
//
// The background goroutine first runs `retryMaxAttempts` active attempts with
// exponential backoff. If those all fail, it switches to a `slowPollInterval`
// cadence and keeps trying indefinitely. Either way, `Ready()` flips to true
// the moment any fetch succeeds.
func NewJWTValidator(keycloakURL, realm string) *JWTValidator {
	cleanURL := strings.TrimSuffix(keycloakURL, "/")
	v := &JWTValidator{
		keycloakURL: cleanURL,
		issuerURL:   cleanURL, // default; override with SetIssuerURL if needed
		realm:       realm,
		publicKeys:  make(map[string]*rsa.PublicKey),
		unknownKIDs: make(map[string]time.Time),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}

	go v.initLoop()

	log.Info("JWT validator created; initial JWKS fetch running in background",
		"keycloakURL", keycloakURL, "realm", realm)
	return v
}

// initLoop runs the initial JWKS fetch with exponential backoff, then falls
// back to a slow poll if all active attempts fail. It exits as soon as any
// fetch succeeds (subsequent refreshes are driven on-demand by ValidateToken)
// or when Stop() is called.
func (v *JWTValidator) initLoop() {
	defer close(v.doneCh)

	backoff := retryInitialBackoff
	for attempt := 1; attempt <= retryMaxAttempts; attempt++ {
		if v.stopped() {
			return
		}
		err := v.fetchPublicKeys()
		if err == nil {
			v.ready.Store(true)
			log.Info("JWT validator ready", "attempt", attempt)
			return
		}
		log.Info("Failed to fetch Keycloak public keys, retrying",
			"attempt", attempt, "maxRetries", retryMaxAttempts,
			"backoff", backoff, "error", err,
			"hint", "verify KEYCLOAK_URL is correct — Keycloak 17+ does not use /auth as a context root")
		if attempt < retryMaxAttempts {
			retryDelay(backoff)
			backoff *= 2
		}
	}

	// Active budget exhausted. Switch to slow polling so an operator looking at
	// logs sees a steady "still failing" cadence rather than a tight retry
	// storm, but the validator still comes online if Keycloak eventually
	// recovers (e.g. a long-running restart, a misconfigured KEYCLOAK_URL
	// fixed in-place).
	//
	// The sleep is a real time.After rather than retryDelay so tests do not
	// have to override it — a test that overrode retryDelay to a no-op would
	// otherwise see this loop spin tight.
	log.Info("active retry budget exhausted; switching to slow poll",
		"interval", slowPollInterval)
	for {
		select {
		case <-v.stopCh:
			return
		case <-time.After(slowPollInterval):
		}
		err := v.fetchPublicKeys()
		if err == nil {
			v.ready.Store(true)
			log.Info("JWT validator ready (slow poll)")
			return
		}
		log.Info("Slow poll JWKS fetch failed", "error", err)
	}
}

// stopped reports whether Stop() has been called. Used as a fast exit check
// at the top of each active retry iteration.
func (v *JWTValidator) stopped() bool {
	select {
	case <-v.stopCh:
		return true
	default:
		return false
	}
}

// Ready reports whether the initial JWKS fetch has succeeded. Handlers gating
// on Authorization headers should return 503 Service Unavailable while this
// is false so clients can distinguish "auth not online yet" from "your token
// is bad" (401).
func (v *JWTValidator) Ready() bool {
	return v.ready.Load()
}

// Stats returns a snapshot of JWKS refresh and unknown-key counters.
func (v *JWTValidator) Stats() ValidatorStats {
	stats := ValidatorStats{
		Ready:                v.Ready(),
		JWKSRefreshAttempts:  v.jwksRefreshAttempts.Load(),
		JWKSRefreshSuccesses: v.jwksRefreshSuccesses.Load(),
		JWKSRefreshFailures:  v.jwksRefreshFailures.Load(),
		JWKSRefreshSkipped:   v.jwksRefreshSkipped.Load(),
		JWKSRefreshCoalesced: v.jwksRefreshCoalesced.Load(),
		UnknownKIDTotal:      v.unknownKIDTotal.Load(),
		UnknownKIDCacheHits:  v.unknownKIDCacheHits.Load(),
	}

	v.keysMu.RLock()
	stats.KeyCount = len(v.publicKeys)
	stats.LastFetch = v.lastFetch
	v.keysMu.RUnlock()

	v.refreshMu.Lock()
	stats.LastRefreshAttempt = v.lastRefreshAttempt
	v.refreshMu.Unlock()

	v.unknownKIDMu.Lock()
	v.pruneUnknownKIDsLocked(time.Now())
	stats.UnknownKIDCacheEntries = len(v.unknownKIDs)
	v.unknownKIDMu.Unlock()

	return stats
}

// Stop signals the background init goroutine to exit and waits for it.
// Safe to call multiple times, including concurrently from multiple goroutines
// (stopOnce guards against a double-close panic). In production,
// NewJWTValidator's goroutine runs for process lifetime so Stop is mainly for
// tests that need to avoid leaking goroutines.
func (v *JWTValidator) Stop() {
	v.stopOnce.Do(func() { close(v.stopCh) })
	<-v.doneCh
}

// SetIssuerURL overrides the URL used to validate the token's `iss` claim.
// Use this when the Keycloak external URL (written into tokens as "iss") differs
// from the internal cluster URL used for JWK fetching (KEYCLOAK_URL).
// An empty string is a no-op; the validator keeps using keycloakURL for issuer
// validation (default behaviour).
//
// Example:
//
//	v, _ := auth.NewJWTValidator(internalURL, realm)  // JWKs fetched from internal URL
//	v.SetIssuerURL(externalURL)                        // iss validated against external URL
func (v *JWTValidator) SetIssuerURL(url string) {
	if url == "" {
		return
	}
	v.issuerURL = strings.TrimSuffix(url, "/")
}

// clockLeeway is the tolerance applied when validating the "exp" claim.
// oauth2-proxy forwards the raw access token from its session; by the time
// the token traverses nginx and reaches the webapi handler, a few seconds may
// have elapsed. Without leeway, even tiny clock drift or network delay causes
// a spurious "token is expired" error. 30 seconds is generous but still
// provides real expiry protection (Keycloak default access-token lifetime is
// 5 minutes, so a 30s window does not materially weaken security).
const clockLeeway = 30 * time.Second

// ValidateToken validates a JWT token and returns the claims.
// Returns ErrNotReady if the initial JWKS fetch has not yet completed; the
// caller should surface this as 503 Service Unavailable.
func (v *JWTValidator) ValidateToken(tokenString string) (*Claims, error) {
	if !v.ready.Load() {
		return nil, ErrNotReady
	}

	if len(tokenString) > maxJWTBytes {
		return nil, fmt.Errorf("token exceeds maximum size of %d bytes", maxJWTBytes)
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}
		if len(kid) > maxKIDBytes {
			return nil, fmt.Errorf("kid exceeds maximum size of %d bytes", maxKIDBytes)
		}

		v.keysMu.RLock()
		publicKey, exists := v.publicKeys[kid]
		v.keysMu.RUnlock()
		if !exists {
			v.unknownKIDTotal.Add(1)
			if v.unknownKIDCached(kid) {
				return nil, fmt.Errorf("unknown key ID: %s (cached)", kid)
			}

			// Key not cached — try a bounded refresh (Keycloak may have rotated
			// keys), coalesced with other callers and subject to a cooldown.
			if refreshErr := v.refreshPublicKeysIfDue(); refreshErr != nil {
				if errors.Is(refreshErr, errJWKSRefreshSkipped) {
					return nil, fmt.Errorf("unknown key ID %s and key refresh skipped by cooldown: %w", kid, refreshErr)
				}
				v.rememberUnknownKID(kid)
				return nil, fmt.Errorf("unknown key ID %s and key refresh failed: %w", kid, refreshErr)
			}
			v.keysMu.RLock()
			publicKey, exists = v.publicKeys[kid]
			v.keysMu.RUnlock()
			if !exists {
				v.rememberUnknownKID(kid)
				return nil, fmt.Errorf("unknown key ID: %s (not found in current key set)", kid)
			}
		}
		v.keysMu.RLock()
		lastFetch := v.lastFetch
		v.keysMu.RUnlock()
		if lastFetch.IsZero() || time.Since(lastFetch) > jwksStaleRefreshInterval {
			v.refreshPublicKeysIfDueAsync()
		}

		return publicKey, nil
	},
		// Allow clockLeeway on the exp claim so small delays between oauth2-proxy
		// forwarding the token and the webapi validating it do not cause false
		// "token expired" rejections.
		jwt.WithLeeway(clockLeeway),
	)

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	expectedIssuer := fmt.Sprintf("%s/realms/%s", v.issuerURL, v.realm)
	if claims.Issuer != expectedIssuer {
		return nil, fmt.Errorf("invalid issuer: expected %s, got %s", expectedIssuer, claims.Issuer)
	}

	// Note: expiry is already enforced by jwt.ParseWithClaims above (with clockLeeway).
	// No redundant manual check needed.

	return claims, nil
}

func (v *JWTValidator) fetchPublicKeys() error {
	v.jwksRefreshAttempts.Add(1)
	if err := v.fetchPublicKeysOnce(); err != nil {
		v.jwksRefreshFailures.Add(1)
		return err
	}
	v.jwksRefreshSuccesses.Add(1)
	return nil
}

func (v *JWTValidator) fetchPublicKeysOnce() error {
	certsURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", v.keycloakURL, v.realm)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", certsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch keys: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error(err, "Failed to close response body")
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch keys: status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	newPublicKeys := make(map[string]*rsa.PublicKey)

	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		publicKey, err := parseRSAPublicKey(jwk)
		if err != nil {
			log.Error(err, "Failed to parse RSA public key", "kid", jwk.Kid)
			continue
		}

		newPublicKeys[jwk.Kid] = publicKey
		log.Info("Loaded public key", "kid", jwk.Kid)
	}

	if len(newPublicKeys) == 0 {
		return fmt.Errorf("no valid RSA keys found")
	}

	v.keysMu.Lock()
	v.publicKeys = newPublicKeys
	v.lastFetch = time.Now()
	v.keysMu.Unlock()

	log.Info("Public keys refreshed", "count", len(newPublicKeys))
	return nil
}

func (v *JWTValidator) refreshPublicKeysIfDue() error {
	call, skipped := v.startPublicKeysRefresh()
	if skipped {
		return errJWKSRefreshSkipped
	}
	if call == nil {
		return nil
	}
	<-call.done
	v.refreshMu.Lock()
	err := call.err
	v.refreshMu.Unlock()
	return err
}

func (v *JWTValidator) refreshPublicKeysIfDueAsync() {
	call, _ := v.startPublicKeysRefresh()
	if call == nil {
		return
	}

	go func() {
		<-call.done
		v.refreshMu.Lock()
		err := call.err
		v.refreshMu.Unlock()
		if err != nil {
			log.Error(err, "Failed to refresh public keys")
		}
	}()
}

func (v *JWTValidator) startPublicKeysRefresh() (*jwksRefreshCall, bool) {
	now := time.Now()
	v.keysMu.RLock()
	lastFetch := v.lastFetch
	v.keysMu.RUnlock()

	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()

	if v.refreshCall != nil {
		v.jwksRefreshCoalesced.Add(1)
		return v.refreshCall, false
	}
	if !v.lastRefreshAttempt.IsZero() && now.Sub(v.lastRefreshAttempt) < jwksRefreshMinInterval {
		v.jwksRefreshSkipped.Add(1)
		return nil, true
	}
	if !lastFetch.IsZero() && now.Sub(lastFetch) < jwksRefreshMinInterval {
		v.jwksRefreshSkipped.Add(1)
		return nil, true
	}

	v.lastRefreshAttempt = now
	call := &jwksRefreshCall{done: make(chan struct{})}
	v.refreshCall = call
	ch := v.refreshGroup.DoChan("jwks-refresh", func() (interface{}, error) {
		return nil, v.fetchPublicKeys()
	})

	go func() {
		res := <-ch
		v.refreshMu.Lock()
		call.err = res.Err
		if v.refreshCall == call {
			v.refreshCall = nil
		}
		close(call.done)
		v.refreshMu.Unlock()
	}()

	return call, false
}

func (v *JWTValidator) unknownKIDCached(kid string) bool {
	v.unknownKIDMu.Lock()
	defer v.unknownKIDMu.Unlock()

	expiresAt, ok := v.unknownKIDs[kid]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(v.unknownKIDs, kid)
		return false
	}
	v.unknownKIDCacheHits.Add(1)
	return true
}

func (v *JWTValidator) rememberUnknownKID(kid string) {
	if unknownKIDCacheTTL <= 0 || unknownKIDCacheMaxEntries <= 0 {
		return
	}

	now := time.Now()
	v.unknownKIDMu.Lock()
	defer v.unknownKIDMu.Unlock()

	v.pruneUnknownKIDsLocked(now)
	if len(v.unknownKIDs) >= unknownKIDCacheMaxEntries {
		for cachedKID := range v.unknownKIDs {
			delete(v.unknownKIDs, cachedKID)
			break
		}
	}
	if v.unknownKIDs == nil {
		v.unknownKIDs = make(map[string]time.Time)
	}
	v.unknownKIDs[kid] = now.Add(unknownKIDCacheTTL)
}

func (v *JWTValidator) pruneUnknownKIDsLocked(now time.Time) {
	for kid, expiresAt := range v.unknownKIDs {
		if now.After(expiresAt) {
			delete(v.unknownKIDs, kid)
		}
	}
}

func parseRSAPublicKey(jwk JWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e*256 + int(b)
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}
