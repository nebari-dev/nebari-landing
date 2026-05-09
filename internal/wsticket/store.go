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
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ticketTTL = 30 * time.Second
	keyPrefix = "wsticket:"
)

// Store issues and redeems single-use WebSocket authentication tickets.
type Store struct {
	rdb *redis.Client
}

// NewStore creates a Store backed by the given Redis client.
func NewStore(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

// Issue mints a random single-use ticket associated with username. The ticket
// expires after 30 seconds whether or not it is redeemed.
func (s *Store) Issue(ctx context.Context, username string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("wsticket: generate ticket: %w", err)
	}
	ticket := hex.EncodeToString(b)
	if err := s.rdb.Set(ctx, keyPrefix+ticket, username, ticketTTL).Err(); err != nil {
		return "", fmt.Errorf("wsticket: store ticket: %w", err)
	}
	return ticket, nil
}

// Redeem validates ticket, removes it from the store (making it single-use),
// and returns the associated username. Returns an error when the ticket is
// unknown, expired, or has already been redeemed.
func (s *Store) Redeem(ctx context.Context, ticket string) (string, error) {
	username, err := s.rdb.GetDel(ctx, keyPrefix+ticket).Result()
	if err != nil {
		if err == redis.Nil {
			return "", fmt.Errorf("wsticket: unknown or expired ticket")
		}
		return "", fmt.Errorf("wsticket: redeem: %w", err)
	}
	return username, nil
}
