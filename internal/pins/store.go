// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package pins provides a Redis-backed persistent store for per-user
// pinned-service preferences in the service-discovery API.
//
// Data model:
//
//	key: nebari:pins:{userID}  (Redis Set of service UIDs)
//
// Operations are idempotent by nature of Redis Set semantics —
// SADD and SREM are both no-ops when the element is already present/absent.
package pins

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "nebari:pins:"

// PinStore persists per-user pinned service UIDs using Redis Sets.
type PinStore struct {
	rdb *redis.Client
}

// NewPinStore returns a PinStore backed by the given Redis client.
// The client must already be configured; no connection is verified here.
func NewPinStore(rdb *redis.Client) *PinStore {
	return &PinStore{rdb: rdb}
}

// Close is a no-op; the Redis client lifetime is managed by the caller.
func (s *PinStore) Close() error { return nil }

// Get returns the list of pinned UIDs for userID.
// Returns an empty slice when the user has no pins stored yet.
func (s *PinStore) Get(userID string) ([]string, error) {
	uids, err := s.rdb.SMembers(context.Background(), pinKey(userID)).Result()
	if err != nil {
		return nil, fmt.Errorf("pins.Get %q: %w", userID, err)
	}
	if uids == nil {
		uids = []string{}
	}
	return uids, nil
}

// Pin adds uid to userID's pinned set (idempotent).
func (s *PinStore) Pin(userID, uid string) error {
	if err := s.rdb.SAdd(context.Background(), pinKey(userID), uid).Err(); err != nil {
		return fmt.Errorf("pins.Pin %q → %q: %w", userID, uid, err)
	}
	return nil
}

// Unpin removes uid from userID's pinned set (idempotent).
func (s *PinStore) Unpin(userID, uid string) error {
	if err := s.rdb.SRem(context.Background(), pinKey(userID), uid).Err(); err != nil {
		return fmt.Errorf("pins.Unpin %q → %q: %w", userID, uid, err)
	}
	return nil
}

func pinKey(userID string) string {
	return keyPrefix + userID
}
