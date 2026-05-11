// Package wsticket provides a Redis-backed single-use ticket store for
// WebSocket authentication. Browsers cannot send Authorization headers on
// WebSocket upgrade requests, so the frontend calls POST /api/v1/ws-ticket
// (authenticated normally with Bearer) to obtain a short-lived opaque ticket,
// then opens the WebSocket with ?ticket=<id> as a query parameter.
package wsticket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ticketTTL = 30 * time.Second
	keyPrefix = "wsticket:"
	// Stored value is empty — the existence of the key is the only signal
	// callers care about. We previously stored the issuing user's username
	// but no caller read it back, so it has been removed.
	storedValue = ""
)

// Store issues and redeems single-use WebSocket authentication tickets.
type Store struct {
	rdb *redis.Client
}

// NewStore creates a Store backed by the given Redis client.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Issue mints a random single-use ticket. The ticket expires after 30
// seconds whether or not it is redeemed.
func (s *Store) Issue(ctx context.Context) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("wsticket: generate ticket: %w", err)
	}
	ticket := hex.EncodeToString(b)
	if err := s.rdb.Set(ctx, keyPrefix+ticket, storedValue, ticketTTL).Err(); err != nil {
		return "", fmt.Errorf("wsticket: store ticket: %w", err)
	}
	return ticket, nil
}

// Redeem validates ticket and removes it from the store (making it
// single-use). Returns an error when the ticket is unknown, expired, or has
// already been redeemed.
func (s *Store) Redeem(ctx context.Context, ticket string) error {
	_, err := s.rdb.GetDel(ctx, keyPrefix+ticket).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("wsticket: unknown or expired ticket")
		}
		return fmt.Errorf("wsticket: redeem: %w", err)
	}
	return nil
}
