// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package useridentity

import "testing"

func TestStableID(t *testing.T) {
	got := StableID("https://keycloak.example/realms/main", "subject-1")
	want := "iss:aHR0cHM6Ly9rZXljbG9hay5leGFtcGxlL3JlYWxtcy9tYWlu:sub:c3ViamVjdC0x"
	if got != want {
		t.Fatalf("StableID() = %q, want %q", got, want)
	}
	if got == StableID("https://keycloak.example/realms/other", "subject-1") {
		t.Fatal("different issuers must produce different stable IDs")
	}
	if got == StableID("https://keycloak.example/realms/main", "subject-2") {
		t.Fatal("different subjects must produce different stable IDs")
	}
	if StableID("", "subject-1") != "" || StableID("issuer", "") != "" {
		t.Fatal("missing issuer or subject should not produce a stable ID")
	}
}
