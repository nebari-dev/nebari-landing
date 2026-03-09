package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("jwt-validator")

// retryMaxAttempts and retryInitialBackoff control the JWKS fetch retry loop in
// NewJWTValidator. They are package-level variables so tests can override them
// without incurring real sleep time.
var (
	retryMaxAttempts    = 5
	retryInitialBackoff = 2 * time.Second
	// retryDelay is called between attempts; replaced in tests to avoid sleeping.
	retryDelay = time.Sleep
)

// Claims represents the JWT claims we care about
type Claims struct {
	jwt.RegisteredClaims
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Groups            []string `json:"groups"`
}

// JWTValidator validates JWT tokens from Keycloak
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

// NewJWTValidator creates a new JWT validator.
// It retries the initial JWKS fetch with exponential backoff so that transient
// Keycloak unavailability (e.g. slow startup, rolling restarts) does not crash
// the service. retryMaxAttempts attempts are made before returning an error.
func NewJWTValidator(keycloakURL, realm string) (*JWTValidator, error) {
	cleanURL := strings.TrimSuffix(keycloakURL, "/")
	v := &JWTValidator{
		keycloakURL: cleanURL,
		issuerURL:   cleanURL, // default; override with SetIssuerURL if needed
		realm:       realm,
		publicKeys:  make(map[string]*rsa.PublicKey),
	}

	backoff := retryInitialBackoff
	var lastErr error
	for attempt := 1; attempt <= retryMaxAttempts; attempt++ {
		lastErr = v.fetchPublicKeys()
		if lastErr == nil {
			break
		}
		log.Info("Failed to fetch Keycloak public keys, retrying",
			"attempt", attempt, "maxRetries", retryMaxAttempts,
			"backoff", backoff, "error", lastErr,
			"hint", "verify KEYCLOAK_URL includes the context path (e.g. /auth for Keycloak X)")
		if attempt < retryMaxAttempts {
			retryDelay(backoff)
			backoff *= 2
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("failed to fetch public keys after %d attempts: %w", retryMaxAttempts, lastErr)
	}

	log.Info("JWT validator initialized", "keycloakURL", keycloakURL, "realm", realm)
	return v, nil
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

// ValidateToken validates a JWT token and returns the claims
func (v *JWTValidator) ValidateToken(tokenString string) (*Claims, error) {
	if time.Since(v.lastFetch) > time.Hour {
		if err := v.fetchPublicKeys(); err != nil {
			log.Error(err, "Failed to refresh public keys")
		}
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

		v.keysMu.RLock()
		defer v.keysMu.RUnlock()

		publicKey, exists := v.publicKeys[kid]
		if !exists {
			// Key not cached — try a one-shot refresh (Keycloak may have rotated keys).
			v.keysMu.RUnlock()
			if refreshErr := v.fetchPublicKeys(); refreshErr != nil {
				v.keysMu.RLock()
				return nil, fmt.Errorf("unknown key ID %s and key refresh failed: %w", kid, refreshErr)
			}
			v.keysMu.RLock()
			publicKey, exists = v.publicKeys[kid]
			if !exists {
				return nil, fmt.Errorf("unknown key ID: %s (not found after key refresh)", kid)
			}
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

	v.keysMu.Lock()
	defer v.keysMu.Unlock()

	v.publicKeys = make(map[string]*rsa.PublicKey)

	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}

		publicKey, err := parseRSAPublicKey(jwk)
		if err != nil {
			log.Error(err, "Failed to parse RSA public key", "kid", jwk.Kid)
			continue
		}

		v.publicKeys[jwk.Kid] = publicKey
		log.Info("Loaded public key", "kid", jwk.Kid)
	}

	v.lastFetch = time.Now()

	if len(v.publicKeys) == 0 {
		return fmt.Errorf("no valid RSA keys found")
	}

	log.Info("Public keys refreshed", "count", len(v.publicKeys))
	return nil
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
