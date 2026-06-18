// Package wsticket provides a Redis-backed single-use ticket store for
// WebSocket authentication. Browsers cannot send Authorization headers on
// WebSocket upgrade requests, so the frontend calls POST /api/v1/ws-ticket
// (authenticated normally with Bearer) to obtain a short-lived ticket and
// then opens the WebSocket with ?ticket=<id> as a query parameter.
//
// The ticket value carries a claims snapshot. An earlier revision of this
// package stripped identity from the stored value because no caller read it
// back; that decision is explicitly retracted here. Issue #95 (per-client
// access filter on the WebSocket fan-out) needs the upgrade handler to learn
// the caller's identity so the hub can apply the same `canAccessService`
// policy that the REST endpoints already enforce. The Bearer WS path has
// the live JWT and re-derives claims itself; the ticket path has nothing
// after redemption, hence the snapshot.
//
// Operational note: the ticket value contains a short-lived (30s)
// authorization snapshot — Subject, PreferredUsername, Groups, and the
// JWT ExpiresAt — that lets the upgrade handler re-derive a Principal
// without re-touching the JWT validator. Group names in this product
// encode organizational structure (admin, data-science, finance, …). A
// Redis dump within the TTL window reveals which users are online and
// their group memberships, which is information disclosure — not
// credential theft (none of these values can be replayed to authenticate
// as the user). Even so, treat the backing Redis instance as
// session-data-sensitive: TLS on the wire and access restricted to the
// webapi ServiceAccount.
package wsticket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ticketTTL = 30 * time.Second
	keyPrefix = "wsticket:"
)

// ErrUnknownTicket is returned by Redeem when the ticket id is not present in
// the store — either it was never issued, has already been redeemed, or has
// expired. Wrapped with `%w`, callers can match via errors.Is.
var ErrUnknownTicket = errors.New("wsticket: unknown or expired ticket")

// TicketClaims is the per-caller snapshot stored alongside the ticket id.
// It carries the minimum the hub needs to apply the service access policy
// (`Subject` for logging/audit, `Groups` for membership checks) plus the
// JWT expiration so the hub can bound the WS session at the same point the
// underlying token would have expired.
type TicketClaims struct {
	Subject           string    `json:"sub"`
	PreferredUsername string    `json:"preferred_username,omitempty"`
	Groups            []string  `json:"groups,omitempty"`
	ExpiresAt         time.Time `json:"exp"`
}

// Store issues and redeems single-use WebSocket authentication tickets.
type Store struct {
	rdb *redis.Client
}

// NewStore creates a Store backed by the given Redis client.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Issue mints a random single-use ticket bound to the supplied claims
// snapshot. The ticket expires after 30 seconds whether or not it is
// redeemed; that is independent of `tc.ExpiresAt`, which bounds the WS
// session if the ticket is redeemed in time.
func (s *Store) Issue(ctx context.Context, tc TicketClaims) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("wsticket: generate ticket: %w", err)
	}
	ticket := hex.EncodeToString(b)
	payload, err := json.Marshal(tc)
	if err != nil {
		return "", fmt.Errorf("wsticket: marshal claims: %w", err)
	}
	if err := s.rdb.Set(ctx, keyPrefix+ticket, payload, ticketTTL).Err(); err != nil {
		return "", fmt.Errorf("wsticket: store ticket: %w", err)
	}
	return ticket, nil
}

// Redeem validates the ticket, removes it from the store (making it
// single-use), and returns the claims snapshot that was supplied at Issue
// time. Returns ErrUnknownTicket when the ticket is unknown, expired, or
// already redeemed.
func (s *Store) Redeem(ctx context.Context, ticket string) (TicketClaims, error) {
	raw, err := s.rdb.GetDel(ctx, keyPrefix+ticket).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return TicketClaims{}, ErrUnknownTicket
		}
		return TicketClaims{}, fmt.Errorf("wsticket: redeem: %w", err)
	}
	var tc TicketClaims
	if err := json.Unmarshal([]byte(raw), &tc); err != nil {
		return TicketClaims{}, fmt.Errorf("wsticket: unmarshal claims: %w", err)
	}
	return tc, nil
}
