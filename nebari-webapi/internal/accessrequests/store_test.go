// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package accessrequests

import (
	"errors"
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "ar.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

const (
	svcUID  = "svc-uid-1"
	svcName = "grafana"
	userA   = "alice"
	userB   = "bob"
)

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
	if _, err := s.UpdateStatus(req.ID, StatusDenied, "admin"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
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
	s.Create(svcUID, svcName, userB, "", "")        //nolint:errcheck
	s.UpdateStatus(req.ID, StatusApproved, "admin") //nolint:errcheck
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

// --- UpdateStatus ---

func TestUpdateStatus_Approve_SetsFields(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	updated, err := s.UpdateStatus(req.ID, StatusApproved, "admin-user")
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

func TestUpdateStatus_Deny_SetsFields(t *testing.T) {
	s := newStore(t)
	req, _ := s.Create(svcUID, svcName, userA, "", "")
	updated, err := s.UpdateStatus(req.ID, StatusDenied, "admin-user")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if updated.Status != StatusDenied {
		t.Errorf("expected denied, got %q", updated.Status)
	}
}

func TestUpdateStatus_NotFound_ReturnsError(t *testing.T) {
	s := newStore(t)
	_, err := s.UpdateStatus("does-not-exist", StatusApproved, "admin")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateStatus_PersistedOnReopen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ar.db")
	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("open s1: %v", err)
	}
	req, _ := s1.Create(svcUID, svcName, userA, "", "")
	s1.UpdateStatus(req.ID, StatusApproved, "admin") //nolint:errcheck
	s1.Close()                                       //nolint:errcheck

	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("open s2: %v", err)
	}
	defer s2.Close() //nolint:errcheck
	got, err := s2.Get(req.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Status != StatusApproved {
		t.Errorf("expected approved status after reopen, got %q", got.Status)
	}
}
