// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package accessrequests

import (
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb)
}

const (
	svcUID  = "svc-uid-1"
	svcName = "grafana"
	userA   = "alice"
	userB   = "bob"
)

func testIdentity(id, email string) UserIdentity {
	return UserIdentity{ID: id, Email: email}
}

// --- Create ---

func TestCreate_HappyPath(t *testing.T) {
	s := newStore(t)
	req, err := s.Create(svcUID, svcName, testIdentity(userA, "alice@example.com"), "please")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if req.Status != StatusPending {
		t.Errorf("expected pending, got %q", req.Status)
	}
	if req.ServiceUID != svcUID {
		t.Errorf("expected serviceUID %q, got %q", svcUID, req.ServiceUID)
	}
	if req.UserID != userA {
		t.Errorf("expected userID %q, got %q", userA, req.UserID)
	}
}

func TestCreate_PersistsImmutableIdentityAndDisplayUsername(t *testing.T) {
	s := newStore(t)
	req, err := s.Create(svcUID, svcName, UserIdentity{
		ID:       "stable-user-key",
		Issuer:   "https://keycloak.example/realms/main",
		Subject:  "keycloak-user-id",
		Username: "alice",
		Email:    "alice@example.com",
	}, "please")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get(req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != "stable-user-key" {
		t.Errorf("expected stable user key, got %q", got.UserID)
	}
	if got.UserIssuer != "https://keycloak.example/realms/main" || got.UserSubject != "keycloak-user-id" {
		t.Errorf("expected immutable issuer/subject, got issuer=%q subject=%q", got.UserIssuer, got.UserSubject)
	}
	if got.Username != "alice" || got.UserEmail != "alice@example.com" {
		t.Errorf("expected display metadata, got username=%q email=%q", got.Username, got.UserEmail)
	}
}

func TestCreate_DuplicatePending_ReturnsError(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create(svcUID, svcName, testIdentity(userA, ""), ""); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := s.Create(svcUID, svcName, testIdentity(userA, ""), "please again")
	if !errors.Is(err, ErrDuplicatePending) {
		t.Errorf("expected ErrDuplicatePending, got %v", err)
	}
}

func TestCreate_DifferentUsers_BothAllowed(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create(svcUID, svcName, testIdentity(userA, ""), ""); err != nil {
		t.Fatalf("alice Create: %v", err)
	}
	if _, err := s.Create(svcUID, svcName, testIdentity(userB, ""), ""); err != nil {
		t.Fatalf("bob Create: %v", err)
	}
}

func TestCreate_DifferentServices_BothAllowed(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create(svcUID, svcName, testIdentity(userA, ""), ""); err != nil {
		t.Fatalf("svc1 Create: %v", err)
	}
	if _, err := s.Create("svc-uid-2", "mlflow", testIdentity(userA, ""), ""); err != nil {
		t.Fatalf("svc2 Create: %v", err)
	}
}

func TestCreate_AfterResolved_AllowsNewPending(t *testing.T) {
	s := newStore(t)
	req, err := s.Create(svcUID, svcName, testIdentity(userA, ""), "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.UpdateStatus(req.ID, StatusDenied, UserIdentity{ID: "admin"}); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	// Now a new pending request should be accepted since old one is resolved.
	if _, err := s.Create(svcUID, svcName, testIdentity(userA, ""), "please try again"); err != nil {
		t.Fatalf("second Create after denial: %v", err)
	}
}

// --- Get ---

func TestGet_Existing(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, testIdentity(userA, ""), "")
	got, err := s.Get(req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != req.ID {
		t.Errorf("expected ID %q, got %q", req.ID, got.ID)
	}
}

func TestGet_NotFound_ReturnsError(t *testing.T) {
	s := newStore(t)
	_, err := s.Get("does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// --- ListAll ---

func TestListAll_Empty_ReturnsEmptySlice(t *testing.T) {
	s := newStore(t)
	all, err := s.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 results, got %d", len(all))
	}
}

func TestListAll_ReturnsAllRequests(t *testing.T) {
	s := newStore(t)
	s.Create(svcUID, svcName, testIdentity(userA, ""), "") //nolint:errcheck
	s.Create(svcUID, svcName, testIdentity(userB, ""), "") //nolint:errcheck
	all, err := s.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 results, got %d", len(all))
	}
}

// --- ListPending ---

func TestListPending_ExcludesResolved(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, testIdentity(userA, ""), "")
	s.Create(svcUID, svcName, testIdentity(userB, ""), "")            //nolint:errcheck
	s.UpdateStatus(req.ID, StatusApproved, UserIdentity{ID: "admin"}) //nolint:errcheck
	pending, err := s.ListPending()
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].UserID != userB {
		t.Errorf("expected bob's pending request, got %q", pending[0].UserID)
	}
}

// --- ListForUser ---

func TestListForUser_ReturnsOnlyUserRequests(t *testing.T) {
	s := newStore(t)
	s.Create(svcUID, svcName, testIdentity(userA, ""), "")   //nolint:errcheck
	s.Create("svc-2", "mlflow", testIdentity(userA, ""), "") //nolint:errcheck
	s.Create(svcUID, svcName, testIdentity(userB, ""), "")   //nolint:errcheck
	reqs, err := s.ListForUser(userA)
	if err != nil {
		t.Fatalf("ListForUser: %v", err)
	}
	if len(reqs) != 2 {
		t.Errorf("expected 2 requests for alice, got %d", len(reqs))
	}
	for _, r := range reqs {
		if r.UserID != userA {
			t.Errorf("unexpected userID %q in alice's list", r.UserID)
		}
	}
}

// --- UpdateStatus ---

func TestUpdateStatus_Approve_SetsFields(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, testIdentity(userA, ""), "")
	updated, err := s.UpdateStatus(req.ID, StatusApproved, UserIdentity{ID: "admin-user"})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != StatusApproved {
		t.Errorf("expected approved, got %q", updated.Status)
	}
	if updated.ResolvedBy != "admin-user" {
		t.Errorf("expected resolvedBy 'admin-user', got %q", updated.ResolvedBy)
	}
	if updated.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
}

func TestUpdateStatus_PersistsImmutableResolverIdentity(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, testIdentity(userA, ""), "")
	updated, err := s.UpdateStatus(req.ID, StatusApproved, UserIdentity{
		ID:       "stable-admin-key",
		Issuer:   "https://keycloak.example/realms/main",
		Subject:  "admin-user-id",
		Username: "admin",
	})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.ResolvedBy != "stable-admin-key" {
		t.Errorf("expected stable resolver key, got %q", updated.ResolvedBy)
	}
	if updated.ResolvedByIssuer != "https://keycloak.example/realms/main" || updated.ResolvedBySubject != "admin-user-id" {
		t.Errorf("expected resolver issuer/subject, got issuer=%q subject=%q", updated.ResolvedByIssuer, updated.ResolvedBySubject)
	}
	if updated.ResolvedByUsername != "admin" {
		t.Errorf("expected resolver display username, got %q", updated.ResolvedByUsername)
	}
}

func TestUpdateStatus_Deny_SetsFields(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, testIdentity(userA, ""), "")
	updated, err := s.UpdateStatus(req.ID, StatusDenied, UserIdentity{ID: "admin-user"})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != StatusDenied {
		t.Errorf("expected denied, got %q", updated.Status)
	}
}

func TestUpdateStatus_NotFound_ReturnsError(t *testing.T) {
	s := newStore(t)
	_, err := s.UpdateStatus("does-not-exist", StatusApproved, UserIdentity{ID: "admin"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateStatus_PersistedAcrossTwoClients(t *testing.T) {
	// Verify status update visible from a second Store client on the same Redis.
	mr := miniredis.RunT(t)
	rdb1 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb1.Close() })
	s1 := NewStore(rdb1)
	req, _ := s1.Create(svcUID, svcName, testIdentity(userA, ""), "")
	_, _ = s1.UpdateStatus(req.ID, StatusApproved, UserIdentity{ID: "admin"})

	rdb2 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb2.Close() })
	s2 := NewStore(rdb2)
	got, err := s2.Get(req.ID)
	if err != nil {
		t.Fatalf("Get via second client: %v", err)
	}
	if got.Status != StatusApproved {
		t.Errorf("expected approved from second client, got %q", got.Status)
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
	s := NewStore(rdb)

	mr.SetError("server error")

	_, err := s.Create(svcUID, svcName, testIdentity(userA, "alice@example.com"), "please")
	if err == nil {
		t.Error("expected error when Redis is unavailable, got nil")
	}
}

func TestGet_RedisDown_ReturnsError(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	s := NewStore(rdb)

	mr.SetError("server error")

	_, err := s.Get("any-id")
	if err == nil {
		t.Error("expected error when Redis is unavailable, got nil")
	}
}
