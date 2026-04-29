// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package wsticket_test

import (
	"context"
	"strings"
	"testing"

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

// --- Issue ---

func TestIssue_ReturnsNonEmptyHexTicket(t *testing.T) {
	store, _ := newStore(t)
	ticket, err := store.Issue(context.Background(), "alice")
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
	t1, err := store.Issue(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	t2, err := store.Issue(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if t1 == t2 {
		t.Error("expected two Issue calls to produce different tickets")
	}
}

// --- Redeem ---

func TestRedeem_ValidTicket_ReturnsUsername(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	ticket, err := store.Issue(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.Redeem(ctx, ticket)
	if err != nil {
		t.Fatalf("unexpected error redeeming valid ticket: %v", err)
	}
	if got != "alice" {
		t.Errorf("expected username %q, got %q", "alice", got)
	}
}

func TestRedeem_UnknownTicket_Errors(t *testing.T) {
	store, _ := newStore(t)
	_, err := store.Redeem(context.Background(), "deadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Fatal("expected error for unknown ticket, got nil")
	}
}

func TestRedeem_SingleUse_SecondRedeemErrors(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	ticket, err := store.Issue(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// First redeem succeeds.
	if _, err := store.Redeem(ctx, ticket); err != nil {
		t.Fatalf("first redeem failed: %v", err)
	}
	// Second redeem must fail — ticket was deleted by the first call.
	if _, err := store.Redeem(ctx, ticket); err == nil {
		t.Fatal("expected error on second redeem of a used ticket, got nil")
	}
}

func TestRedeem_ExpiredTicket_Errors(t *testing.T) {
	store, mr := newStore(t)
	ctx := context.Background()
	ticket, err := store.Issue(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// Jump miniredis clock past the 30 s TTL.
	mr.FastForward(31e9) // 31 seconds in nanoseconds
	_, err = store.Redeem(ctx, ticket)
	if err == nil {
		t.Fatal("expected error for expired ticket, got nil")
	}
}

func TestRedeem_EmptyTicket_Errors(t *testing.T) {
	store, _ := newStore(t)
	_, err := store.Redeem(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty ticket, got nil")
	}
}

func TestIssue_DifferentUsersGetDifferentTickets(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()
	ta, _ := store.Issue(ctx, "alice")
	tb, _ := store.Issue(ctx, "bob")
	if ta == tb {
		t.Error("different users received identical tickets")
	}
	// Ensure each ticket resolves to the correct owner.
	ua, err := store.Redeem(ctx, ta)
	if err != nil {
		t.Fatal(err)
	}
	ub, err := store.Redeem(ctx, tb)
	if err != nil {
		t.Fatal(err)
	}
	if ua != "alice" {
		t.Errorf("expected alice, got %q", ua)
	}
	if ub != "bob" {
		t.Errorf("expected bob, got %q", ub)
	}
}
