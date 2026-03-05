// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package pins

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const testUID1 = "uid-1"

func newStore(t *testing.T) *PinStore {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewPinStore(rdb)
}

// --- Get ---

func TestGet_NewUser_ReturnsEmptySlice(t *testing.T) {
	s := newStore(t)
	uids, err := s.Get("alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(uids) != 0 {
		t.Errorf("expected empty slice, got %v", uids)
	}
}

// --- Pin ---

func TestPin_AddsUID(t *testing.T) {
	s := newStore(t)
	if err := s.Pin("alice", testUID1); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	uids, _ := s.Get("alice")
	if len(uids) != 1 || uids[0] != testUID1 {
		t.Errorf("expected [uid-1], got %v", uids)
	}
}

func TestPin_Idempotent(t *testing.T) {
	s := newStore(t)
	_ = s.Pin("alice", testUID1)
	_ = s.Pin("alice", testUID1)
	uids, _ := s.Get("alice")
	if len(uids) != 1 {
		t.Errorf("expected 1 pin after idempotent Pin, got %v", uids)
	}
}

func TestPin_MultipleUIDs(t *testing.T) {
	s := newStore(t)
	_ = s.Pin("alice", testUID1)
	_ = s.Pin("alice", "uid-2")
	_ = s.Pin("alice", "uid-3")
	uids, _ := s.Get("alice")
	if len(uids) != 3 {
		t.Errorf("expected 3 pins, got %v", uids)
	}
}

func TestPin_IsolatedPerUser(t *testing.T) {
	s := newStore(t)
	_ = s.Pin("alice", testUID1)
	_ = s.Pin("bob", "uid-2")
	alice, _ := s.Get("alice")
	bob, _ := s.Get("bob")
	if len(alice) != 1 || alice[0] != testUID1 {
		t.Errorf("alice: expected [uid-1], got %v", alice)
	}
	if len(bob) != 1 || bob[0] != "uid-2" {
		t.Errorf("bob: expected [uid-2], got %v", bob)
	}
}

// --- Unpin ---

func TestUnpin_RemovesUID(t *testing.T) {
	s := newStore(t)
	_ = s.Pin("alice", testUID1)
	_ = s.Pin("alice", "uid-2")
	if err := s.Unpin("alice", testUID1); err != nil {
		t.Fatalf("Unpin: %v", err)
	}
	uids, _ := s.Get("alice")
	if len(uids) != 1 || uids[0] != "uid-2" {
		t.Errorf("expected [uid-2], got %v", uids)
	}
}

func TestUnpin_Idempotent(t *testing.T) {
	s := newStore(t)
	_ = s.Pin("alice", testUID1)
	_ = s.Unpin("alice", testUID1)
	_ = s.Unpin("alice", testUID1) // second unpin must not error
	uids, _ := s.Get("alice")
	if len(uids) != 0 {
		t.Errorf("expected empty after unpin, got %v", uids)
	}
}

func TestUnpin_NonExistentUser_Noop(t *testing.T) {
	s := newStore(t)
	if err := s.Unpin("ghost", testUID1); err != nil {
		t.Errorf("unpin non-existent user should not error: %v", err)
	}
}

func TestUnpin_DoesNotAffectOtherUsers(t *testing.T) {
	s := newStore(t)
	_ = s.Pin("alice", testUID1)
	_ = s.Pin("bob", testUID1)
	_ = s.Unpin("alice", testUID1)
	bob, _ := s.Get("bob")
	if len(bob) != 1 || bob[0] != testUID1 {
		t.Errorf("bob's pins should be unaffected: got %v", bob)
	}
}

// --- Persistence ---

func TestPersistence_SameRedis_RetainsData(t *testing.T) {
	// With Redis, data persists as long as the server is running.
	// Verify two separate PinStore clients pointing at the same server share state.
	mr := miniredis.RunT(t)
	rdb1 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb1.Close() })
	s1 := NewPinStore(rdb1)
	_ = s1.Pin("alice", testUID1)

	rdb2 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb2.Close() })
	s2 := NewPinStore(rdb2)
	uids, err := s2.Get("alice")
	if err != nil {
		t.Fatalf("Get via second client: %v", err)
	}
	if len(uids) != 1 || uids[0] != testUID1 {
		t.Errorf("expected [uid-1] from second client, got %v", uids)
	}
}

// --- Close ---

func TestClose_ReturnsNoError(t *testing.T) {
	s := newStore(t)
	if err := s.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}
