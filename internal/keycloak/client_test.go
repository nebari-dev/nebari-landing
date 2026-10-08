// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package keycloak

import (
	"reflect"
	"testing"

	"github.com/Nerzal/gocloak/v13"
)

func TestCollectGroupPaths_IndexesSubgroupLeaves(t *testing.T) {
	groups := []*gocloak.Group{
		{
			Name: gocloak.StringP("division-a"),
			Path: gocloak.StringP("/division-a"),
			SubGroups: &[]gocloak.Group{
				{Name: gocloak.StringP("research"), Path: gocloak.StringP("/division-a/research")},
			},
		},
		{
			Name: gocloak.StringP("division-b"),
			Path: gocloak.StringP("/division-b"),
			SubGroups: &[]gocloak.Group{
				{Name: gocloak.StringP("research"), Path: gocloak.StringP("/division-b/research")},
			},
		},
	}
	byLeaf := map[string][]string{}
	collectGroupPaths(groups, byLeaf)

	want := []string{"/division-a/research", "/division-b/research"}
	if !reflect.DeepEqual(byLeaf["research"], want) {
		t.Fatalf("research paths: got %v, want %v", byLeaf["research"], want)
	}
}

func TestCollectGroupPaths_FallsBackToRootPathFromName(t *testing.T) {
	groups := []*gocloak.Group{
		{Name: gocloak.StringP("admin")},
	}
	byLeaf := map[string][]string{}
	collectGroupPaths(groups, byLeaf)

	want := []string{"/admin"}
	if !reflect.DeepEqual(byLeaf["admin"], want) {
		t.Fatalf("admin paths: got %v, want %v", byLeaf["admin"], want)
	}
}

func TestClientInternalID_RequiresExactClientID(t *testing.T) {
	clients := []*gocloak.Client{
		{ID: gocloak.StringP("internal-1"), ClientID: gocloak.StringP("landing")},
		{ID: gocloak.StringP("internal-2"), ClientID: gocloak.StringP("landing-extra")},
	}

	got, ok := clientInternalID(clients, "landing")
	if !ok || got != "internal-1" {
		t.Fatalf("clientInternalID exact: got %q, %v; want internal-1, true", got, ok)
	}
}

func TestClientInternalID_RejectsMissingID(t *testing.T) {
	clients := []*gocloak.Client{
		{ClientID: gocloak.StringP("landing")},
	}

	got, ok := clientInternalID(clients, "landing")
	if ok || got != "" {
		t.Fatalf("clientInternalID missing id: got %q, %v; want empty, false", got, ok)
	}
}
