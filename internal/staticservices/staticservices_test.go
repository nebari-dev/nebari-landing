// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package staticservices

import (
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-landing/internal/cache"
)

const keycloakJSON = `[{
	"id": "keycloak",
	"displayName": "Keycloak",
	"description": "Identity and access administration",
	"url": "https://keycloak.example.com/admin/nebari/console/",
	"category": "Platform",
	"priority": 90,
	"visibility": "private",
	"requiredGroups": ["/keycloak-admins"],
	"healthCheck": {"url": "http://keycloak-keycloakx-http.keycloak.svc:8080/realms/nebari"}
}]`

func TestParse_Empty(t *testing.T) {
	for _, in := range []string{"", "  \n", "[]"} {
		got, err := Parse([]byte(in))
		if err != nil {
			t.Fatalf("Parse(%q): unexpected error %v", in, err)
		}
		if len(got) != 0 {
			t.Fatalf("Parse(%q): expected no services, got %d", in, len(got))
		}
	}
}

func TestParse_Keycloak(t *testing.T) {
	got, err := Parse([]byte(keycloakJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got))
	}
	s := got[0]
	if s.ID != "keycloak" || s.Visibility != "private" {
		t.Fatalf("unexpected service: %+v", s)
	}
	if len(s.RequiredGroups) != 1 || s.RequiredGroups[0] != "/keycloak-admins" {
		t.Fatalf("requiredGroups = %v", s.RequiredGroups)
	}
}

func TestParse_DefaultsVisibilityToPrivate(t *testing.T) {
	got, err := Parse([]byte(`[{"id":"docs","url":"https://docs.example.com"}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].Visibility != "private" {
		t.Fatalf("visibility = %q, want private", got[0].Visibility)
	}
}

func TestParse_Errors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"not json", `{`, "decoding"},
		{"object not array", `{"id":"a"}`, "decoding"},
		{"unknown field", `[{"id":"a","url":"https://a.example.com","requiredGroup":["/x"]}]`, "unknown field"},
		{"missing id", `[{"url":"https://a.example.com"}]`, "id must be"},
		{"bad id", `[{"id":"Key Cloak","url":"https://a.example.com"}]`, "id must be"},
		{"missing url", `[{"id":"a"}]`, "url"},
		{"relative url", `[{"id":"a","url":"/admin"}]`, "absolute http(s)"},
		{"javascript url", `[{"id":"a","url":"javascript:alert(1)"}]`, "absolute http(s)"},
		{"bad visibility", `[{"id":"a","url":"https://a.example.com","visibility":"admins"}]`, "visibility must be"},
		{"public with groups", `[{"id":"a","url":"https://a.example.com","visibility":"public","requiredGroups":["/x"]}]`, "no effect"},
		{"bad health url", `[{"id":"a","url":"https://a.example.com","healthCheck":{"url":"keycloak:8080"}}]`, "healthCheck.url"},
		{"duplicate id", `[{"id":"a","url":"https://a.example.com"},{"id":"a","url":"https://b.example.com"}]`, "duplicate id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.in))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestApp_PopulatesCacheEntry(t *testing.T) {
	services, err := Parse([]byte(keycloakJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := cache.NewServiceCache()
	c.Add(services[0].App())

	got := c.Get(UIDPrefix + "keycloak")
	if got == nil {
		t.Fatal("expected static entry in cache")
	}
	if !got.Static {
		t.Error("expected Static=true")
	}
	if got.URL != "https://keycloak.example.com/admin/nebari/console/" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.Visibility != "private" || len(got.RequiredGroups) != 1 || got.RequiredGroups[0] != "/keycloak-admins" {
		t.Errorf("visibility/requiredGroups = %q/%v", got.Visibility, got.RequiredGroups)
	}
	if got.Priority != 90 || got.Category != "Platform" || got.DisplayName != "Keycloak" {
		t.Errorf("display fields = %q/%q/%d", got.DisplayName, got.Category, got.Priority)
	}
	if got.HealthCheckConfig == nil {
		t.Fatal("expected a health check config")
	}
	if got.HealthCheckConfig.ProbeURL != "http://keycloak-keycloakx-http.keycloak.svc:8080/realms/nebari" {
		t.Errorf("ProbeURL = %q", got.HealthCheckConfig.ProbeURL)
	}
	if got.HealthCheckConfig.IntervalSeconds != 30 || got.HealthCheckConfig.TimeoutSeconds != 5 {
		t.Errorf("probe defaults = %d/%d", got.HealthCheckConfig.IntervalSeconds, got.HealthCheckConfig.TimeoutSeconds)
	}
}

func TestApp_NoHealthCheck_DisplayNameFallsBackToID(t *testing.T) {
	services, err := Parse([]byte(`[{"id":"docs","url":"https://docs.example.com","visibility":"public"}]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := cache.NewServiceCache()
	c.Add(services[0].App())

	got := c.Get(UIDPrefix + "docs")
	if got == nil {
		t.Fatal("expected static entry in cache")
	}
	if got.HealthCheckConfig != nil {
		t.Errorf("expected no health check, got %+v", got.HealthCheckConfig)
	}
	if got.DisplayName != "docs" {
		t.Errorf("DisplayName = %q, want docs", got.DisplayName)
	}
}
