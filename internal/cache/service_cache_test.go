// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"strings"
	"testing"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
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

func TestBuildHealthCheckConfig_UsesDeclaredSameNamespaceBackend(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-hc", "app", "team-a", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled:         true,
			Path:            "/ready?full=1",
			IntervalSeconds: 60,
			TimeoutSeconds:  7,
		},
	})
	a.ServiceName = "app-backend"
	a.ServicePort = 8080

	c.Add(a)

	svc := c.Get("uid-hc")
	if svc == nil || svc.HealthCheckConfig == nil {
		t.Fatalf("expected health check config, got %+v", svc)
	}
	if got, want := svc.HealthCheckConfig.ProbeURL, "http://app-backend.team-a:8080/ready?full=1"; got != want {
		t.Errorf("ProbeURL = %q, want %q", got, want)
	}
}

func TestBuildHealthCheckConfig_DeniesCrossNamespaceByDefault(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-cross", "app", "team-a", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
		},
	})
	a.ServiceName = "shared-api"
	a.ServiceNamespace = "platform"
	a.ServicePort = 8080

	c.Add(a)

	svc := c.Get("uid-cross")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected cross-namespace health check to be denied, got %+v", svc.HealthCheckConfig)
	}
}

func TestBuildHealthCheckConfig_AllowsConsentedCrossNamespaceTarget(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-cross-ok", "app", "team-a", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
		},
	})
	a.ServiceName = "shared-api"
	a.ServiceNamespace = "platform"
	a.ServicePort = 8080
	a.HealthCheckCrossNamespaceAllowed = true

	c.Add(a)

	svc := c.Get("uid-cross-ok")
	if svc == nil || svc.HealthCheckConfig == nil {
		t.Fatalf("expected health check config, got %+v", svc)
	}
	if got, want := svc.HealthCheckConfig.ProbeURL, "http://shared-api.platform:8080/ready"; got != want {
		t.Errorf("ProbeURL = %q, want %q", got, want)
	}
}

func TestBuildHealthCheckConfig_RejectsInvalidPath(t *testing.T) {
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "relative", path: "ready"},
		{name: "authority", path: "//metadata.internal/latest"},
		{name: "bad escape", path: "/bad%zz"},
		{name: "control character", path: "/bad\nheader"},
		{name: "too long", path: "/" + strings.Repeat("a", maxHealthCheckPathLength)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := NewServiceCache()
			c.Add(makeApp("uid-path", "app", "ns", "app.example.com", &sdapp.LandingPage{
				Enabled: true,
				HealthCheck: &sdapp.HealthCheck{
					Enabled: true,
					Path:    tt.path,
				},
			}))

			svc := c.Get("uid-path")
			if svc == nil {
				t.Fatal("expected service in cache")
			}
			if svc.HealthCheckConfig != nil {
				t.Fatalf("expected invalid path %q to disable health check, got %+v", tt.path, svc.HealthCheckConfig)
			}
		})
	}
}

func TestBuildHealthCheckConfig_ClampsIntervalAndTimeout(t *testing.T) {
	for _, tt := range []struct {
		name         string
		interval     int
		timeout      int
		wantInterval int
		wantTimeout  int
	}{
		{
			name:         "defaults",
			wantInterval: defaultHealthCheckIntervalSeconds,
			wantTimeout:  defaultHealthCheckTimeoutSeconds,
		},
		{
			name:         "minimum interval",
			interval:     1,
			timeout:      5,
			wantInterval: minHealthCheckIntervalSeconds,
			wantTimeout:  5,
		},
		{
			name:         "maximum interval and timeout",
			interval:     999,
			timeout:      999,
			wantInterval: maxHealthCheckIntervalSeconds,
			wantTimeout:  maxHealthCheckTimeoutSeconds,
		},
		{
			name:         "timeout cannot exceed interval",
			interval:     10,
			timeout:      30,
			wantInterval: 10,
			wantTimeout:  10,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := NewServiceCache()
			c.Add(makeApp("uid-bounds", "app", "ns", "app.example.com", &sdapp.LandingPage{
				Enabled: true,
				HealthCheck: &sdapp.HealthCheck{
					Enabled:         true,
					Path:            "/ready",
					IntervalSeconds: tt.interval,
					TimeoutSeconds:  tt.timeout,
				},
			}))

			svc := c.Get("uid-bounds")
			if svc == nil || svc.HealthCheckConfig == nil {
				t.Fatalf("expected health check config, got %+v", svc)
			}
			if got := svc.HealthCheckConfig.IntervalSeconds; got != tt.wantInterval {
				t.Errorf("IntervalSeconds = %d, want %d", got, tt.wantInterval)
			}
			if got := svc.HealthCheckConfig.TimeoutSeconds; got != tt.wantTimeout {
				t.Errorf("TimeoutSeconds = %d, want %d", got, tt.wantTimeout)
			}
		})
	}
}

func TestBuildHealthCheckConfig_RejectsInvalidPort(t *testing.T) {
	c := NewServiceCache()
	c.Add(makeApp("uid-port", "app", "ns", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
			Port:    70000,
		},
	}))

	svc := c.Get("uid-port")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected invalid port to disable health check, got %+v", svc.HealthCheckConfig)
	}
}

func TestBuildHealthCheckConfig_RejectsInvalidServiceDNSName(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-service-name", "app", "ns", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
		},
	})
	a.ServiceName = "bad_name"
	c.Add(a)

	svc := c.Get("uid-service-name")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected invalid service name to disable health check, got %+v", svc.HealthCheckConfig)
	}
}

func TestBuildHealthCheckConfig_RejectsInvalidServiceNamespace(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-service-namespace", "app", "ns", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
		},
	})
	a.ServiceNamespace = "bad_namespace"
	a.HealthCheckCrossNamespaceAllowed = true
	c.Add(a)

	svc := c.Get("uid-service-namespace")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected invalid service namespace to disable health check, got %+v", svc.HealthCheckConfig)
	}
}

func TestBuildHealthCheckConfig_RejectsProtectedTargets(t *testing.T) {
	for _, tt := range []struct {
		name             string
		namespace        string
		serviceName      string
		servicePort      int
		crossNamespaceOK bool
	}{
		{
			name:             "system namespace",
			namespace:        "kube-system",
			serviceName:      "app-backend",
			servicePort:      8080,
			crossNamespaceOK: true,
		},
		{
			name:             "identity namespace",
			namespace:        "keycloak",
			serviceName:      "app-backend",
			servicePort:      8080,
			crossNamespaceOK: true,
		},
		{
			name:        "database service name",
			namespace:   "team-a",
			serviceName: "nebari-redis-master",
			servicePort: 8080,
		},
		{
			name:        "control-plane service name",
			namespace:   "team-a",
			serviceName: "kubernetes",
			servicePort: 8080,
		},
		{
			name:        "database port",
			namespace:   "team-a",
			serviceName: "app-backend",
			servicePort: 5432,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := NewServiceCache()
			a := makeApp("uid-protected", "app", "team-a", "app.example.com", &sdapp.LandingPage{
				Enabled: true,
				HealthCheck: &sdapp.HealthCheck{
					Enabled: true,
					Path:    "/ready",
				},
			})
			a.ServiceNamespace = tt.namespace
			a.ServiceName = tt.serviceName
			a.ServicePort = tt.servicePort
			a.HealthCheckCrossNamespaceAllowed = tt.crossNamespaceOK

			c.Add(a)

			svc := c.Get("uid-protected")
			if svc == nil {
				t.Fatal("expected service in cache")
			}
			if svc.HealthCheckConfig != nil {
				t.Fatalf("expected protected target to disable health check, got %+v", svc.HealthCheckConfig)
			}
		})
	}
}

func TestBuildHealthCheckConfig_RejectsHealthCheckPortOverride(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-port-override", "app", "team-a", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
			Port:    9000,
		},
	})
	a.ServiceName = "app-backend"
	a.ServicePort = 8080

	c.Add(a)

	svc := c.Get("uid-port-override")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected healthCheck.port override to disable health check, got %+v", svc.HealthCheckConfig)
	}
}

func TestAdd_ResetsHealthWhenConfiguredHealthCheckBecomesInvalid(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-reset", "app", "ns", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
		},
	})
	c.Add(a)
	now := time.Now()
	c.UpdateHealth("uid-reset", &HealthStatus{Status: "healthy", LastCheck: &now})

	a.ServiceNamespace = "other-ns"
	c.Add(a)

	svc := c.Get("uid-reset")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected denied health check config, got %+v", svc.HealthCheckConfig)
	}
	if svc.Health == nil || svc.Health.Status != "unknown" || svc.Health.LastCheck != nil {
		t.Fatalf("expected health to reset to unknown, got %+v", svc.Health)
	}
}

func TestAdd_ResetsHealthWhenHealthCheckIsRemoved(t *testing.T) {
	c := NewServiceCache()
	a := makeApp("uid-remove-health", "app", "ns", "app.example.com", &sdapp.LandingPage{
		Enabled: true,
		HealthCheck: &sdapp.HealthCheck{
			Enabled: true,
			Path:    "/ready",
		},
	})
	c.Add(a)
	now := time.Now()
	c.UpdateHealth("uid-remove-health", &HealthStatus{Status: "healthy", LastCheck: &now})

	a.LandingPage.HealthCheck = nil
	c.Add(a)

	svc := c.Get("uid-remove-health")
	if svc == nil {
		t.Fatal("expected service in cache")
	}
	if svc.HealthCheckConfig != nil {
		t.Fatalf("expected removed health check config, got %+v", svc.HealthCheckConfig)
	}
	if svc.Health == nil || svc.Health.Status != "unknown" || svc.Health.LastCheck != nil {
		t.Fatalf("expected health to reset to unknown, got %+v", svc.Health)
	}
}
