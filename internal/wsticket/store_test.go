// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package wsticket_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/nebari-dev/nebari-landing/internal/wsticket"
)

func newStore(t *testing.T) (*wsticket.Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return wsticket.NewStore(rdb), mr
}

func sampleClaims() wsticket.TicketClaims {
	return wsticket.TicketClaims{
		Subject:           "user-uuid-1",
		PreferredUsername: "alice",
		Groups:            []string{"admin", "data-science"},
		ExpiresAt:         time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

// --- Issue ---

func TestIssue_ReturnsNonEmptyHexTicket(t *testing.T) {
	store, _ := newStore(t)
	ticket, err := store.Issue(context.Background(), sampleClaims())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket == "" {
		t.Fatal("expected non-empty ticket")
	}
	// 16 random bytes → 32 hex characters
	if len(ticket) != 32 {
		t.Errorf("expected 32-char hex ticket, got %q (len %d)", ticket, len(ticket))
	}
	for _, c := range ticket {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("ticket contains non-hex character %q in %q", c, ticket)
			break
		}
	}
}

func TestIssue_EachCallReturnsUniqueTicket(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	t1, err := store.Issue(ctx, sampleClaims())
	if err != nil {
		t.Fatal(err)
	}
	t2, err := store.Issue(ctx, sampleClaims())
	if err != nil {
		t.Fatal(err)
	}
	if t1 == t2 {
		t.Error("expected two Issue calls to produce different tickets")
	}
}

// --- Redeem ---

func TestRedeem_ValidTicket_RoundtripsClaims(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	in := sampleClaims()
	ticket, err := store.Issue(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := store.Redeem(ctx, ticket)
	if err != nil {
		t.Fatalf("unexpected error redeeming valid ticket: %v", err)
	}
	if out.Subject != in.Subject {
		t.Errorf("Subject: expected %q, got %q", in.Subject, out.Subject)
	}
	if out.PreferredUsername != in.PreferredUsername {
		t.Errorf("PreferredUsername: expected %q, got %q", in.PreferredUsername, out.PreferredUsername)
	}
	if len(out.Groups) != len(in.Groups) {
		t.Fatalf("Groups length: expected %d, got %d (%v)", len(in.Groups), len(out.Groups), out.Groups)
	}
	for i, g := range in.Groups {
		if out.Groups[i] != g {
			t.Errorf("Groups[%d]: expected %q, got %q", i, g, out.Groups[i])
		}
	}
	if !out.ExpiresAt.Equal(in.ExpiresAt) {
		t.Errorf("ExpiresAt: expected %v, got %v", in.ExpiresAt, out.ExpiresAt)
	}
}

func TestRedeem_UnknownTicket_ReturnsErrUnknownTicket(t *testing.T) {
	store, _ := newStore(t)
	_, err := store.Redeem(context.Background(), "deadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("expected error for unknown ticket, got nil")
	}
	if !errors.Is(err, wsticket.ErrUnknownTicket) {
		t.Errorf("expected ErrUnknownTicket, got %v", err)
	}
}

func TestRedeem_SingleUse_SecondRedeemErrors(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	ticket, err := store.Issue(ctx, sampleClaims())
	if err != nil {
		t.Fatal(err)
	}
	// First redeem succeeds.
	if _, err := store.Redeem(ctx, ticket); err != nil {
		t.Fatalf("first redeem failed: %v", err)
	}
	// Second redeem must fail — ticket was deleted by the first call.
	_, err = store.Redeem(ctx, ticket)
	if err == nil {
		t.Fatal("expected error on second redeem of a used ticket, got nil")
	}
	if !errors.Is(err, wsticket.ErrUnknownTicket) {
		t.Errorf("expected ErrUnknownTicket on second redeem, got %v", err)
	}
}

func TestRedeem_ExpiredTicket_Errors(t *testing.T) {
	store, mr := newStore(t)
	ctx := context.Background()
	ticket, err := store.Issue(ctx, sampleClaims())
	if err != nil {
		t.Fatal(err)
	}
	// Jump miniredis clock past the 30 s TTL.
	mr.FastForward(31e9) // 31 seconds in nanoseconds
	_, err = store.Redeem(ctx, ticket)
	if err == nil {
		t.Fatal("expected error for expired ticket, got nil")
	}
	if !errors.Is(err, wsticket.ErrUnknownTicket) {
		t.Errorf("expected ErrUnknownTicket for expired ticket, got %v", err)
	}
}

func TestRedeem_EmptyTicket_Errors(t *testing.T) {
	store, _ := newStore(t)
	_, err := store.Redeem(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ticket, got nil")
	}
}

// TestIssueRedeem_NilGroupsAndEmptyGroups_RoundtripIdentically covers the
// edge case where Issue is called with Groups: nil vs Groups: []string{}.
// JSON marshal+unmarshal collapses both to a zero-length slice (nil for
// omitempty fields), so downstream policy code (which uses len(groups))
// treats them identically. Locking this in via a test means a future change
// to the TicketClaims tags or to hasRequiredGroups can't quietly diverge.
func TestIssueRedeem_NilGroupsAndEmptyGroups_RoundtripIdentically(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	// Issue with nil Groups.
	t1, err := store.Issue(ctx, wsticket.TicketClaims{Subject: "alice", Groups: nil, ExpiresAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	tc1, err := store.Redeem(ctx, t1)
	if err != nil {
		t.Fatal(err)
	}

	// Issue with empty Groups.
	t2, err := store.Issue(ctx, wsticket.TicketClaims{Subject: "alice", Groups: []string{}, ExpiresAt: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	tc2, err := store.Redeem(ctx, t2)
	if err != nil {
		t.Fatal(err)
	}

	if len(tc1.Groups) != 0 {
		t.Errorf("nil Groups roundtripped to len=%d, expected 0", len(tc1.Groups))
	}
	if len(tc2.Groups) != 0 {
		t.Errorf("empty Groups roundtripped to len=%d, expected 0", len(tc2.Groups))
	}
}

func TestRedeem_MalformedStoredValue_Errors(t *testing.T) {
	// Forward-compat: if a value somehow ends up in Redis that isn't a valid
	// JSON-encoded TicketClaims (manual injection, schema mismatch, etc.),
	// Redeem must reject it cleanly rather than returning a zero-value
	// TicketClaims that the caller might silently treat as "valid anonymous."
	store, mr := newStore(t)
	ctx := context.Background()
	// Write a bogus value directly under the expected key.
	if err := mr.Set("wsticket:badtick", "this is not json"); err != nil {
		t.Fatalf("miniredis Set: %v", err)
	}
	_, err := store.Redeem(ctx, "badtick")
	if err == nil {
		t.Fatal("expected error for malformed stored value, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal claims") {
		t.Errorf("expected unmarshal-claims error, got %v", err)
	}
}
