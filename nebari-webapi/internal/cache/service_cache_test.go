// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"testing"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/nebari-webapi/internal/app"
)

// makeApp constructs an sdapp.App for use in cache tests.
// lp may be nil to simulate an app with no landing-page config.
func makeApp(uid, name, ns, hostname string, lp *sdapp.LandingPage) *sdapp.App {
	return &sdapp.App{
		UID:         uid,
		Name:        name,
		Namespace:   ns,
		Hostname:    hostname,
		TLSEnabled:  true,
		LandingPage: lp,
	}
}

func TestAdd_NilLandingPage_NoEntry(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-1", "app", "ns", "app.example.com", nil))
	if got := c.Get("uid-1"); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestAdd_DisabledLandingPage_RemovesExisting(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-1", "app", "ns", "app.example.com",
		&sdapp.LandingPage{Enabled: true, DisplayName: "App"}))
	if c.Get("uid-1") == nil {
		t.Fatal("expected service to be cached after Add with Enabled=true")
	}
	c.Add(makeApp("uid-1", "app", "ns", "app.example.com",
		&sdapp.LandingPage{Enabled: false}))
	if got := c.Get("uid-1"); got != nil {
		t.Fatalf("expected nil after disabling, got %+v", got)
	}
}

func TestAdd_Enabled_StoresCorrectFields(t *testing.T) {
	c := NewServiceCache()
	lp := &sdapp.LandingPage{
		Enabled:        true,
		DisplayName:    "My App",
		Description:    "A test app",
		Icon:           "jupyter",
		Category:       "Development",
		Priority:       42,
		Visibility:     "public",
		RequiredGroups: []string{"admins"},
		ExternalURL:    "https://external.example.com",
	}
	c.Add(makeApp("uid-2", "myapp", "default", "myapp.example.com", lp))
	svc := c.Get("uid-2")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	checks := map[string][2]interface{}{
		"UID":         {"uid-2", svc.UID},
		"Name":        {"myapp", svc.Name},
		"Namespace":   {"default", svc.Namespace},
		"DisplayName": {"My App", svc.DisplayName},
		"Description": {"A test app", svc.Description},
		"Icon":        {"jupyter", svc.Icon},
		"Category":    {"Development", svc.Category},
		"Priority":    {42, svc.Priority},
		"Visibility":  {"public", svc.Visibility},
		"URL":         {"https://external.example.com", svc.URL},
	}
	for name, v := range checks {
		if v[0] != v[1] {
			t.Errorf("%s: want %v, got %v", name, v[0], v[1])
		}
	}
	if len(svc.RequiredGroups) != 1 || svc.RequiredGroups[0] != "admins" {
		t.Errorf("RequiredGroups: got %v, want [admins]", svc.RequiredGroups)
	}
}

func TestAdd_DefaultPriority(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-3", "app", "ns", "app.example.com",
		&sdapp.LandingPage{Enabled: true, Priority: 100}))
	if svc := c.Get("uid-3"); svc.Priority != 100 {
		t.Errorf("expected priority 100, got %d", svc.Priority)
	}
}

func TestAdd_DefaultVisibility(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-4", "app", "ns", "app.example.com",
		&sdapp.LandingPage{Enabled: true, Visibility: "authenticated"}))
	if svc := c.Get("uid-4"); svc.Visibility != "authenticated" {
		t.Errorf("expected default visibility 'authenticated', got %q", svc.Visibility)
	}
}

func TestRemove(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-5", "app", "ns", "app.example.com",
		&sdapp.LandingPage{Enabled: true}))
	c.Remove("uid-5")
	if svc := c.Get("uid-5"); svc != nil {
		t.Fatalf("expected nil after Remove, got %+v", svc)
	}
}

func TestRemove_NonExistentUID_Noop(t *testing.T) {
	c := NewServiceCache()
	c.Remove("does-not-exist")
}

func TestBuildURL_ExternalURL(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-u1", "app", "ns", "app.example.com",
		&sdapp.LandingPage{Enabled: true, ExternalURL: "https://custom.example.com/path"}))
	if svc := c.Get("uid-u1"); svc.URL != "https://custom.example.com/path" {
		t.Errorf("expected ExternalURL, got %q", svc.URL)
	}
}

func TestBuildURL_DefaultHTTPS(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-u2", "app", "ns", "myapp.example.com",
		&sdapp.LandingPage{Enabled: true}))
	if svc := c.Get("uid-u2"); svc.URL != "https://myapp.example.com" {
		t.Errorf("expected https URL, got %q", svc.URL)
	}
}

func TestBuildURL_TLSDisabled_HTTP(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-u3", "app", "ns", "myapp.example.com",
		&sdapp.LandingPage{Enabled: true})
	a.TLSEnabled = false
	c.Add(a)
	if svc := c.Get("uid-u3"); svc.URL != "http://myapp.example.com" {
		t.Errorf("expected http URL, got %q", svc.URL)
	}
}

func TestGetAll_SortsByPriorityThenName(t *testing.T) {
	c := NewServiceCache()
	for _, a := range []struct {
		uid, name string
		prio      int
	}{
		{"u3", "zepth", 10},
		{"u1", "alpha", 50},
		{"u2", "beta", 50},
		{"u4", "first", 1},
	} {
		lp := &sdapp.LandingPage{Enabled: true, Priority: a.prio}
		c.Add(makeApp(a.uid, a.name, "ns", "h.example.com", lp))
	}
	all := c.GetAll()
	if len(all) != 4 {
		t.Fatalf("expected 4, got %d", len(all))
	}
	for i, want := range []string{"first", "zepth", "alpha", "beta"} {
		if all[i].Name != want {
			t.Errorf("pos %d: got %q, want %q", i, all[i].Name, want)
		}
	}
}

func TestGetAll_EmptyCache(t *testing.T) {
	c := NewServiceCache()
	if all := c.GetAll(); len(all) != 0 {
		t.Errorf("expected empty slice, got %d items", len(all))
	}
}

func TestGetCategories_UniqueAndSorted(t *testing.T) {
	c := NewServiceCache()
	for i, cat := range []string{"Monitoring", "Development", "Monitoring", "Platform"} {
		uid := "uid-cat-" + string(rune(48+i))
		c.Add(makeApp(uid, "app", "ns", "h.com",
			&sdapp.LandingPage{Enabled: true, Category: cat}))
	}
	cats := c.GetCategories()
	want := []string{"Development", "Monitoring", "Platform"}
	if len(cats) != len(want) {
		t.Fatalf("expected %v, got %v", want, cats)
	}
	for i, cat := range cats {
		if cat != want[i] {
			t.Errorf("pos %d: got %q, want %q", i, cat, want[i])
		}
	}
}

func TestGetCategories_EmptyCategory_Excluded(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-nc", "app", "ns", "h.com",
		&sdapp.LandingPage{Enabled: true, Category: ""}))
	if cats := c.GetCategories(); len(cats) != 0 {
		t.Errorf("expected no categories, got %v", cats)
	}
}

func TestUpdateHealth_ExistingService(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-h", "app", "ns", "h.com",
		&sdapp.LandingPage{Enabled: true}))
	now := time.Now()
	c.UpdateHealth("uid-h", &HealthStatus{Status: "healthy", LastCheck: &now, Message: "OK"})
	svc := c.Get("uid-h")
	if svc.Health == nil || svc.Health.Status != "healthy" {
		t.Errorf("expected healthy status, got %v", svc.Health)
	}
}

func TestUpdateHealth_NonExistentUID_Noop(t *testing.T) {
	c := NewServiceCache()
	c.UpdateHealth("does-not-exist", &HealthStatus{Status: "healthy"})
}

func TestAdd_PreservesExistingHealthStatus(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-hp", "app", "ns", "h.com", &sdapp.LandingPage{Enabled: true})
	c.Add(a)
	now := time.Now()
	c.UpdateHealth("uid-hp", &HealthStatus{Status: "healthy", LastCheck: &now})
	a.LandingPage.DisplayName = "Updated"
	c.Add(a)
	svc := c.Get("uid-hp")
	if svc.Health == nil || svc.Health.Status != "healthy" {
		t.Errorf("expected preserved health, got %v", svc.Health)
	}
}

func TestAdd_InitialHealthStatus_Unknown(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-init", "app", "ns", "h.com",
		&sdapp.LandingPage{Enabled: true}))
	svc := c.Get("uid-init")
	if svc.Health == nil || svc.Health.Status != "unknown" {
		t.Errorf("expected initial health 'unknown', got %v", svc.Health)
	}
}

func TestGetByNamespacedName(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-ns1", "grafana", "monitoring", "grafana.example.com",
		&sdapp.LandingPage{Enabled: true}))
	svc := c.GetByNamespacedName("monitoring", "grafana")
	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.UID != "uid-ns1" {
		t.Errorf("got UID %q, want uid-ns1", svc.UID)
	}
}

func TestGetByNamespacedName_NotFound(t *testing.T) {
	c := NewServiceCache()
	if svc := c.GetByNamespacedName("ns", "missing"); svc != nil {
		t.Errorf("expected nil, got %+v", svc)
	}
}
