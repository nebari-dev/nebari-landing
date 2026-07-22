// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package notifications

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb, time.Hour)
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

func TestCreate_SetsTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	retention := time.Hour
	s := NewStore(rdb, retention)

	n, err := s.Create("", "Title", "Body")
	if err != nil {
		t.Fatal(err)
	}
	if ttl := mr.TTL(notifKey(n.ID)); ttl != retention {
		t.Errorf("expected TTL %s, got %s", retention, ttl)
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

func TestReadSet_PersistsAcrossTwoClients(t *testing.T) {
	// Verify read-state shared across two Store clients on the same Redis server.
	mr := miniredis.RunT(t)
	rdb1 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb1.Close() })
	s1 := NewStore(rdb1, time.Hour)
	n, _ := s1.Create("", "T", "M")
	_ = s1.MarkRead("alice", n.ID)

	rdb2 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb2.Close() })
	s2 := NewStore(rdb2, time.Hour)
	rs, _ := s2.ReadSet("alice")
	if !rs[n.ID] {
		t.Error("read state should be visible from a second Store client")
	}
}

// --- Close ---

func TestClose_ReturnsNoError(t *testing.T) {
	s := newStore(t)
	if err := s.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

// --- Redis failure paths ---

func TestCreate_RedisDown_ReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewStore(rdb, time.Hour)

	mr.SetError("server error")

	_, err := s.Create("", "Title", "Body")
	if err == nil {
		t.Error("expected error when Redis is unavailable, got nil")
	}
}

func TestGet_RedisDown_ReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewStore(rdb, time.Hour)

	mr.SetError("server error")

	_, err := s.Get("any-id")
	if err == nil {
		t.Error("expected error when Redis is unavailable, got nil")
	}
}

func TestList_RedisDown_ReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewStore(rdb, time.Hour)

	mr.SetError("server error")

	_, err := s.List()
	if err == nil {
		t.Error("expected error when Redis is unavailable, got nil")
	}
}

func TestMarkRead_RedisDown_ReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewStore(rdb, time.Hour)

	mr.SetError("server error")

	if err := s.MarkRead("alice", "any-id"); err == nil {
		t.Error("expected error when Redis is unavailable, got nil")
	}
}

// --- CreateDraft: source-service metadata roundtrip ---

func TestCreateDraft_TaggedFields_RoundtripViaGet(t *testing.T) {
	s := newStore(t)
	d := Draft{
		Image:          "icon.png",
		Title:          "Private is back",
		Message:        "body",
		ServiceUID:     "uid-private-1",
		Visibility:     "private",
		RequiredGroups: []string{"argocd-admins", "finance"},
	}
	n, err := s.CreateDraft(d)
	if err != nil {
		t.Fatal(err)
	}
	if n.ServiceUID != d.ServiceUID || n.Visibility != d.Visibility {
		t.Errorf("returned: %+v", n)
	}
	if len(n.RequiredGroups) != 2 || n.RequiredGroups[0] != "argocd-admins" || n.RequiredGroups[1] != "finance" {
		t.Errorf("RequiredGroups roundtrip broke: %+v", n.RequiredGroups)
	}

	got, err := s.Get(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ServiceUID != d.ServiceUID {
		t.Errorf("Get ServiceUID: expected %q, got %q", d.ServiceUID, got.ServiceUID)
	}
	if got.Visibility != d.Visibility {
		t.Errorf("Get Visibility: expected %q, got %q", d.Visibility, got.Visibility)
	}
	if len(got.RequiredGroups) != 2 || got.RequiredGroups[0] != "argocd-admins" || got.RequiredGroups[1] != "finance" {
		t.Errorf("Get RequiredGroups roundtrip broke: %+v", got.RequiredGroups)
	}
}

func TestCreateDraft_ZeroFields_UntaggedRoundtrip(t *testing.T) {
	// Untagged draft (admin-broadcast shape): all three source-service fields
	// stay empty after roundtrip. Legacy Create() delegates here and must
	// remain equivalent to a broadcast-to-all notification.
	s := newStore(t)
	n, err := s.CreateDraft(Draft{Image: "", Title: "Welcome", Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ServiceUID != "" || got.Visibility != "" || len(got.RequiredGroups) != 0 {
		t.Errorf("expected untagged notification, got %+v", got)
	}
}

func TestCreate_LegacyShim_MatchesUntaggedDraft(t *testing.T) {
	// Create is retained as a backward-compat shim for admin broadcasts.
	// It must produce a notification indistinguishable from CreateDraft
	// with zero source-service fields.
	s := newStore(t)
	n, err := s.Create("img", "T", "M")
	if err != nil {
		t.Fatal(err)
	}
	if n.ServiceUID != "" || n.Visibility != "" || len(n.RequiredGroups) != 0 {
		t.Errorf("Create must produce an untagged notification, got %+v", n)
	}
}

func TestList_MixedTaggedAndUntagged_PreservesFields(t *testing.T) {
	// Regression guard for the notifFromHash decoder path: List() hits the
	// pipeline HGetAll, not the single-key Get(). If notifFromHash drops the
	// new fields, filtering downstream will silently treat every notification
	// as untagged (broadcast-to-all) - exactly the #170 regression this PR
	// fixes.
	s := newStore(t)
	if _, err := s.CreateDraft(Draft{Title: "public-tagged", ServiceUID: "svc-a", Visibility: "public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDraft(Draft{Title: "private-tagged", ServiceUID: "svc-b", Visibility: "private", RequiredGroups: []string{"g1"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDraft(Draft{Title: "untagged"}); err != nil {
		t.Fatal(err)
	}
	all, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 notifications, got %d", len(all))
	}
	found := map[string]*Notification{}
	for _, n := range all {
		found[n.Title] = n
	}
	if got := found["public-tagged"]; got == nil || got.ServiceUID != "svc-a" || got.Visibility != "public" {
		t.Errorf("public-tagged lost fields: %+v", got)
	}
	if got := found["private-tagged"]; got == nil || got.ServiceUID != "svc-b" || got.Visibility != "private" || len(got.RequiredGroups) != 1 || got.RequiredGroups[0] != "g1" {
		t.Errorf("private-tagged lost fields: %+v", got)
	}
	if got := found["untagged"]; got == nil || got.ServiceUID != "" || got.Visibility != "" || len(got.RequiredGroups) != 0 {
		t.Errorf("untagged shouldn't carry source-service fields: %+v", got)
	}
}
