// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package notifications

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "notif.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// --- Create ---

func TestCreate_ReturnsNotificationWithID(t *testing.T) {
	s := newStore(t)
	n, err := s.Create("img.png", "Hello", "World")
	if err != nil {
		t.Fatal(err)
	}
	if n.ID == "" {
		t.Error("expected non-empty ID")
	}
	if n.Title != "Hello" || n.Message != "World" || n.Image != "img.png" {
		t.Errorf("unexpected fields: %+v", n)
	}
	if n.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestCreate_NoImage_OK(t *testing.T) {
	s := newStore(t)
	n, err := s.Create("", "Title", "Body")
	if err != nil || n.Image != "" {
		t.Errorf("unexpected: err=%v image=%q", err, n.Image)
	}
}

// --- Get ---

func TestGet_ExistingID(t *testing.T) {
	s := newStore(t)
	created, _ := s.Create("", "T", "M")
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID || got.Title != "T" {
		t.Errorf("got %+v", got)
	}
}

func TestGet_UnknownID_ReturnsErrNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Get("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- List ---

func TestList_Empty(t *testing.T) {
	s := newStore(t)
	items, err := s.List()
	if err != nil || len(items) != 0 {
		t.Errorf("expected empty list: err=%v items=%v", err, items)
	}
}

func TestList_NewestFirst(t *testing.T) {
	s := newStore(t)
	for _, title := range []string{"first", "second", "third"} {
		if _, err := s.Create("", title, ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	items, _ := s.List()
	if len(items) != 3 {
		t.Fatalf("expected 3, got %d", len(items))
	}
	if items[0].Title != "third" || items[2].Title != "first" {
		t.Errorf("wrong order: %v %v %v", items[0].Title, items[1].Title, items[2].Title)
	}
}

// --- MarkRead ---

func TestMarkRead_HappyPath(t *testing.T) {
	s := newStore(t)
	n, _ := s.Create("", "T", "M")
	if err := s.MarkRead("alice", n.ID); err != nil {
		t.Fatal(err)
	}
	rs, _ := s.ReadSet("alice")
	if !rs[n.ID] {
		t.Error("notification should be marked read for alice")
	}
}

func TestMarkRead_Idempotent(t *testing.T) {
	s := newStore(t)
	n, _ := s.Create("", "T", "M")
	_ = s.MarkRead("alice", n.ID)
	if err := s.MarkRead("alice", n.ID); err != nil {
		t.Errorf("second MarkRead should be idempotent, got: %v", err)
	}
}

func TestMarkRead_UnknownNotification_ReturnsErrNotFound(t *testing.T) {
	s := newStore(t)
	err := s.MarkRead("alice", "no-such-id")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestMarkRead_IsolatedPerUser(t *testing.T) {
	s := newStore(t)
	n, _ := s.Create("", "T", "M")
	_ = s.MarkRead("alice", n.ID)

	rsAlice, _ := s.ReadSet("alice")
	rsBob, _ := s.ReadSet("bob")

	if !rsAlice[n.ID] {
		t.Error("alice should have read it")
	}
	if rsBob[n.ID] {
		t.Error("bob should NOT have read it")
	}
}

// --- ReadSet ---

func TestReadSet_NewUser_ReturnsEmpty(t *testing.T) {
	s := newStore(t)
	rs, err := s.ReadSet("alice")
	if err != nil || len(rs) != 0 {
		t.Errorf("expected empty: err=%v rs=%v", err, rs)
	}
}

func TestReadSet_MultipleNotifications(t *testing.T) {
	s := newStore(t)
	n1, _ := s.Create("", "A", "")
	n2, _ := s.Create("", "B", "")
	n3, _ := s.Create("", "C", "")
	_ = s.MarkRead("alice", n1.ID)
	_ = s.MarkRead("alice", n3.ID)

	rs, _ := s.ReadSet("alice")
	if !rs[n1.ID] || rs[n2.ID] || !rs[n3.ID] {
		t.Errorf("unexpected read set: %v", rs)
	}
}

func TestReadSet_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	s1, _ := NewStore(filepath.Join(dir, "n.db"))
	n, _ := s1.Create("", "T", "M")
	_ = s1.MarkRead("alice", n.ID)
	_ = s1.Close()

	s2, _ := NewStore(filepath.Join(dir, "n.db"))
	t.Cleanup(func() { _ = s2.Close() })
	rs, _ := s2.ReadSet("alice")
	if !rs[n.ID] {
		t.Error("read state should persist across reopen")
	}
}
