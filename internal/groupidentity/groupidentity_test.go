// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package groupidentity

import "testing"

func TestRequiredPath_RejectsLeafName(t *testing.T) {
	got, ok := RequiredPath("research")
	if ok || got != "" {
		t.Fatalf("RequiredPath leaf: got %q, %v; want empty, false", got, ok)
	}
}

func TestRequiredPath_CleansFullPath(t *testing.T) {
	got, ok := RequiredPath(" /division-a//research ")
	if !ok || got != "/division-a/research" {
		t.Fatalf("RequiredPath full path: got %q, %v; want /division-a/research, true", got, ok)
	}
}

func TestInvalidRequiredPaths_ReportsInvalidValues(t *testing.T) {
	got := InvalidRequiredPaths([]string{"", "research", "/", "/division-a/research", "research", "admin"})
	want := []string{"research", "/", "admin"}
	if len(got) != len(want) {
		t.Fatalf("InvalidRequiredPaths: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("InvalidRequiredPaths[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTokenPath_RejectsLeafOnlyClaim(t *testing.T) {
	if got, ok := TokenPath("research"); ok || got != "" {
		t.Fatalf("TokenPath leaf: got %q, %v; want empty, false", got, ok)
	}
}

func TestHasIntersection_DistinguishesSameLeafDifferentPaths(t *testing.T) {
	if !HasIntersection([]string{"/division-a/research"}, []string{"/division-a/research"}) {
		t.Fatal("expected exact full-path match")
	}
	if HasIntersection([]string{"/division-b/research"}, []string{"/division-a/research"}) {
		t.Fatal("same leaf under another path must not match")
	}
	if HasIntersection([]string{"/division-a/research"}, []string{"research"}) {
		t.Fatal("leaf-only configured group must not match a full-path token group")
	}
}
