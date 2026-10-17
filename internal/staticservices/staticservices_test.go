// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package staticservices

import (
	"os"
	"strings"
	"testing"

	"github.com/nebari-dev/nebari-landing/internal/cache"
)

// keycloakJSON uses non-default values everywhere so a field the conversion
// to the cache drops shows up as a failure rather than as a default.
const keycloakJSON = `[{
	"id": "keycloak",
	"displayName": "Keycloak",
	"description": "Identity and access administration",
	"url": "https://keycloak.example.com/admin/nebari/console/",
	"icon": "https://keycloak.example.com/icon.svg",
	"iconLight": "https://keycloak.example.com/icon-light.svg",
	"iconDark": "https://keycloak.example.com/icon-dark.svg",
	"category": "Platform",
	"priority": 90,
	"visibility": "private",
	"requiredGroups": ["/nebari-admins"],
	"healthCheck": {
		"url": "http://keycloak-keycloakx-http.keycloak.svc:8080/realms/nebari",
		"intervalSeconds": 15,
		"timeoutSeconds": 3
	}
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

// TestParse_ChartOutput parses the entry the Helm chart renders for the
// Keycloak toggle. charts/nebari-landing/tests/assert-static-services.sh checks
// the chart against the same file, so the two cannot drift apart.
func TestParse_ChartOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/chart-keycloak.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("chart output rejected: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got))
	}
	s := got[0]
	if s.ID != "keycloak" || s.Visibility != "private" {
		t.Fatalf("unexpected service: %+v", s)
	}
	if len(s.RequiredGroups) != 1 || s.RequiredGroups[0] != "/nebari-admins" {
		t.Fatalf("requiredGroups = %v", s.RequiredGroups)
	}
	if s.HealthCheck == nil || s.HealthCheck.URL != "http://keycloak.keycloak.svc:8080/realms/nebari" {
		t.Fatalf("healthCheck = %+v", s.HealthCheck)
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
		{"trailing data", `[{"id":"a","url":"https://a.example.com"}] garbage`, "unexpected data"},
		{"two arrays", `[] []`, "unexpected data"},
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
	if got.Visibility != "private" || len(got.RequiredGroups) != 1 || got.RequiredGroups[0] != "/nebari-admins" {
		t.Errorf("visibility/requiredGroups = %q/%v", got.Visibility, got.RequiredGroups)
	}
	if got.Priority != 90 || got.Category != "Platform" || got.DisplayName != "Keycloak" ||
		got.Description != "Identity and access administration" {
		t.Errorf("display fields = %q/%q/%q/%d", got.DisplayName, got.Description, got.Category, got.Priority)
	}
	if got.Icon != "https://keycloak.example.com/icon.svg" ||
		got.IconLight != "https://keycloak.example.com/icon-light.svg" ||
		got.IconDark != "https://keycloak.example.com/icon-dark.svg" {
		t.Errorf("icons = %q/%q/%q", got.Icon, got.IconLight, got.IconDark)
	}
	if got.HealthCheckConfig == nil {
		t.Fatal("expected a health check config")
	}
	if got.HealthCheckConfig.ProbeURL != "http://keycloak-keycloakx-http.keycloak.svc:8080/realms/nebari" {
		t.Errorf("ProbeURL = %q", got.HealthCheckConfig.ProbeURL)
	}
	if got.HealthCheckConfig.IntervalSeconds != 15 || got.HealthCheckConfig.TimeoutSeconds != 3 {
		t.Errorf("probe interval/timeout = %d/%d, want 15/3", got.HealthCheckConfig.IntervalSeconds, got.HealthCheckConfig.TimeoutSeconds)
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
