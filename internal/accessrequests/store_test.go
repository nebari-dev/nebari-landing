// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package accessrequests

import (
	"context"
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
	return NewStore(rdb)
}

const (
	svcUID  = "svc-uid-1"
	svcName = "grafana"
	userA   = "alice"
	userB   = "bob"
)

func resolveForTest(t *testing.T, s *Store, id string, resolution Resolution) *AccessRequest {
	t.Helper()
	updated, err := s.Resolve(id, resolution)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return updated
}

// --- Create ---

func TestCreate_HappyPath(t *testing.T) {
	s := newStore(t)
	req, err := s.Create(svcUID, svcName, userA, "alice@example.com", "please")
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

func TestCreateWithOwner_RecordsTargetOwner(t *testing.T) {
	s := newStore(t)
	req, err := s.CreateWithOwner(svcUID, svcName, "default/grafana", userA, "alice@example.com", "please")
	if err != nil {
		t.Fatalf("CreateWithOwner: %v", err)
	}
	if req.TargetOwner != "default/grafana" {
		t.Fatalf("expected targetOwner, got %q", req.TargetOwner)
	}
	got, err := s.Get(req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.TargetOwner != "default/grafana" {
		t.Fatalf("expected persisted targetOwner, got %q", got.TargetOwner)
	}
}

func TestCreate_DuplicatePending_ReturnsError(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create(svcUID, svcName, userA, "", ""); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := s.Create(svcUID, svcName, userA, "", "please again")
	if !errors.Is(err, ErrDuplicatePending) {
		t.Errorf("expected ErrDuplicatePending, got %v", err)
	}
}

func TestCreate_ActiveEntitlementReturnsError(t *testing.T) {
	s := newStore(t)
	req, err := s.Create(svcUID, svcName, userA, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := s.Resolve(req.ID, Resolution{
		Status:    StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("Resolve approve: %v", err)
	}
	_, err = s.Create(svcUID, svcName, userA, "", "please again")
	if !errors.Is(err, ErrActiveEntitlement) {
		t.Fatalf("expected ErrActiveEntitlement, got %v", err)
	}
}

func TestCreate_DifferentUsers_BothAllowed(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create(svcUID, svcName, userA, "", ""); err != nil {
		t.Fatalf("alice Create: %v", err)
	}
	if _, err := s.Create(svcUID, svcName, userB, "", ""); err != nil {
		t.Fatalf("bob Create: %v", err)
	}
}

func TestCreate_DifferentServices_BothAllowed(t *testing.T) {
	s := newStore(t)
	if _, err := s.Create(svcUID, svcName, userA, "", ""); err != nil {
		t.Fatalf("svc1 Create: %v", err)
	}
	if _, err := s.Create("svc-uid-2", "mlflow", userA, "", ""); err != nil {
		t.Fatalf("svc2 Create: %v", err)
	}
}

func TestCreate_AfterResolved_AllowsNewPending(t *testing.T) {
	s := newStore(t)
	req, err := s.Create(svcUID, svcName, userA, "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	resolveForTest(t, s, req.ID, Resolution{Status: StatusDenied, ResolvedBy: "admin"})
	// Now a new pending request should be accepted since old one is resolved.
	if _, err := s.Create(svcUID, svcName, userA, "", "please try again"); err != nil {
		t.Fatalf("second Create after denial: %v", err)
	}
}

// --- Get ---

func TestGet_Existing(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
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
	s.Create(svcUID, svcName, userA, "", "") //nolint:errcheck
	s.Create(svcUID, svcName, userB, "", "") //nolint:errcheck
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
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	s.Create(svcUID, svcName, userB, "", "") //nolint:errcheck
	resolveForTest(t, s, req.ID, Resolution{Status: StatusApproved, ResolvedBy: "admin"})
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

func TestListByStatus_FiltersResolvedStatuses(t *testing.T) {
	s := newStore(t)
	approved, _ := s.Create(svcUID, svcName, userA, "", "")
	denied, _ := s.Create(svcUID, svcName, userB, "", "")
	revoked, _ := s.Create("svc-2", "mlflow", userA, "", "")
	resolveForTest(t, s, approved.ID, Resolution{Status: StatusApproved, ResolvedBy: "admin"})
	resolveForTest(t, s, denied.ID, Resolution{Status: StatusDenied, ResolvedBy: "admin"})
	resolveForTest(t, s, revoked.ID, Resolution{Status: StatusRevoked, ResolvedBy: "admin"})

	tests := []struct {
		status Status
		wantID string
	}{
		{StatusApproved, approved.ID},
		{StatusDenied, denied.ID},
		{StatusRevoked, revoked.ID},
	}
	for _, tt := range tests {
		got, err := s.ListByStatus(tt.status)
		if err != nil {
			t.Fatalf("ListByStatus(%s): %v", tt.status, err)
		}
		if len(got) != 1 || got[0].ID != tt.wantID {
			t.Fatalf("ListByStatus(%s): got %+v, want only %s", tt.status, got, tt.wantID)
		}
	}
}

// --- ListForUser ---

func TestListForUser_ReturnsOnlyUserRequests(t *testing.T) {
	s := newStore(t)
	s.Create(svcUID, svcName, userA, "", "")   //nolint:errcheck
	s.Create("svc-2", "mlflow", userA, "", "") //nolint:errcheck
	s.Create(svcUID, svcName, userB, "", "")   //nolint:errcheck
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

// --- Resolve ---

func TestResolve_Approve_SetsAuditAndExpiry(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	expiresAt := time.Now().UTC().Add(time.Hour)
	updated, err := s.Resolve(req.ID, Resolution{
		Status:          StatusApproved,
		ResolvedBy:      "admin-subject",
		ResolvedByName:  "Admin User",
		ResolvedByEmail: "admin@example.com",
		ExpiresAt:       &expiresAt,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if updated.Status != StatusApproved {
		t.Errorf("expected approved, got %q", updated.Status)
	}
	if updated.ResolvedBy != "admin-subject" {
		t.Errorf("expected resolvedBy admin-subject, got %q", updated.ResolvedBy)
	}
	if updated.ResolvedByName != "Admin User" {
		t.Errorf("expected resolvedByName, got %q", updated.ResolvedByName)
	}
	if updated.ResolvedByEmail != "admin@example.com" {
		t.Errorf("expected resolvedByEmail, got %q", updated.ResolvedByEmail)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected expiresAt %s, got %v", expiresAt, updated.ExpiresAt)
	}

	got, err := s.Get(req.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected persisted expiresAt %s, got %v", expiresAt, got.ExpiresAt)
	}
}

func TestResolve_Revoke_RemovesActiveApproval(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := s.Resolve(req.ID, Resolution{
		Status:    StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	revoked, err := s.Resolve(req.ID, Resolution{
		Status:     StatusRevoked,
		ResolvedBy: "admin-subject",
	})
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if revoked.Status != StatusRevoked {
		t.Fatalf("expected revoked, got %q", revoked.Status)
	}
	if revoked.ExpiresAt != nil {
		t.Fatalf("expected revoke to clear expiresAt, got %v", revoked.ExpiresAt)
	}
	if revoked.ActiveApproved(time.Now()) {
		t.Fatal("revoked access request must not be active")
	}
	active, err := s.ListActiveServiceUIDs(userA, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListActiveServiceUIDs: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active service grants after revoke, got %+v", active)
	}
}

func TestResolve_RevokeOldRequest_DoesNotRemoveNewerActiveApproval(t *testing.T) {
	s := newStore(t)
	oldReq, _ := s.Create(svcUID, svcName, userA, "", "")
	oldExpiresAt := time.Now().UTC().Add(2 * time.Hour)
	if _, err := s.Resolve(oldReq.ID, Resolution{
		Status:    StatusApproved,
		ExpiresAt: &oldExpiresAt,
	}); err != nil {
		t.Fatalf("approve old request: %v", err)
	}

	// Simulate a historical or repaired state where a second approval becomes
	// current for the same user/service while the old row still says approved.
	if err := s.rdb.Del(context.Background(), arActiveKey(userA, svcUID)).Err(); err != nil {
		t.Fatalf("delete active key: %v", err)
	}
	newReq, err := s.Create(svcUID, svcName, userA, "", "")
	if err != nil {
		t.Fatalf("create new request: %v", err)
	}
	newExpiresAt := time.Now().UTC().Add(3 * time.Hour)
	if _, err := s.Resolve(newReq.ID, Resolution{
		Status:    StatusApproved,
		ExpiresAt: &newExpiresAt,
	}); err != nil {
		t.Fatalf("approve new request: %v", err)
	}

	if _, err := s.Resolve(oldReq.ID, Resolution{Status: StatusRevoked}); err != nil {
		t.Fatalf("revoke old request: %v", err)
	}
	active, err := s.ListActiveServiceUIDs(userA, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListActiveServiceUIDs: %v", err)
	}
	if _, ok := active[svcUID]; !ok {
		t.Fatalf("expected newer active approval to remain, got %+v", active)
	}
}

func TestListActiveServiceUIDs_ReturnsApprovedCurrentEntitlement(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	expiresAt := time.Now().UTC().Add(time.Hour)
	if _, err := s.Resolve(req.ID, Resolution{
		Status:    StatusApproved,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	active, err := s.ListActiveServiceUIDs(userA, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListActiveServiceUIDs: %v", err)
	}
	if _, ok := active[svcUID]; !ok {
		t.Fatalf("expected active grant for %s, got %+v", svcUID, active)
	}
}

func TestActiveApproved_ExpiredReturnsFalse(t *testing.T) {
	past := time.Now().UTC().Add(-time.Second)
	req := &AccessRequest{
		Status:    StatusApproved,
		ExpiresAt: &past,
	}
	if req.ActiveApproved(time.Now().UTC()) {
		t.Fatal("expired approval must not be active")
	}
}

func TestActiveApproved_MissingExpiryReturnsFalse(t *testing.T) {
	req := &AccessRequest{Status: StatusApproved}
	if req.ActiveApproved(time.Now().UTC()) {
		t.Fatal("approval without expiresAt must not be active")
	}
}

func TestResolve_Deny_SetsFields(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	updated := resolveForTest(t, s, req.ID, Resolution{Status: StatusDenied, ResolvedBy: "admin-user"})
	if updated.Status != StatusDenied {
		t.Errorf("expected denied, got %q", updated.Status)
	}
}

func TestResolve_NotFound_ReturnsError(t *testing.T) {
	s := newStore(t)
	_, err := s.Resolve("does-not-exist", Resolution{Status: StatusApproved, ResolvedBy: "admin"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolve_PersistedAcrossTwoClients(t *testing.T) {
	// Verify status update visible from a second Store client on the same Redis.
	mr := miniredis.RunT(t)
	rdb1 := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb1.Close() })
	s1 := NewStore(rdb1)
	req, _ := s1.Create(svcUID, svcName, userA, "", "")
	resolveForTest(t, s1, req.ID, Resolution{Status: StatusApproved, ResolvedBy: "admin"})

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

	_, err := s.Create(svcUID, svcName, userA, "alice@example.com", "please")
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
