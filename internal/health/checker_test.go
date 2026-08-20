package health

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	"github.com/nebari-dev/nebari-landing/internal/cache"
)

func TestProbe_DeniesRedirect(t *testing.T) {
	finalHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			finalHit = true
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	serviceCache := cache.NewServiceCache()
	serviceCache.Add(&sdapp.App{
		UID:         "uid-redirect",
		Name:        "app",
		Namespace:   "ns",
		Hostname:    "app.example.com",
		TLSEnabled:  true,
		LandingPage: &sdapp.LandingPage{Enabled: true},
	})

	checker := NewHealthChecker(serviceCache, time.Second)
	checker.probe(context.Background(), "uid-redirect", server.URL+"/redirect", newProbeHTTPClient(time.Second))

	if finalHit {
		t.Fatal("expected health probe not to follow redirect")
	}
	svc := serviceCache.Get("uid-redirect")
	if svc == nil || svc.Health == nil {
		t.Fatalf("expected health status, got %+v", svc)
	}
	if svc.Health.Status != "unhealthy" {
		t.Fatalf("health status = %q, want unhealthy", svc.Health.Status)
	}
	if svc.Health.Message != "HTTP 302" {
		t.Fatalf("health message = %q, want HTTP 302", svc.Health.Message)
	}
}

func TestProbeIfCurrentSkipsRevokedConfig(t *testing.T) {
	serviceCache := cache.NewServiceCache()
	app := &sdapp.App{
		UID:         "uid-revoked",
		Name:        "app",
		Namespace:   "ns",
		Hostname:    "app.example.com",
		TLSEnabled:  true,
		ServiceName: "app-backend",
		ServicePort: 8080,
		LandingPage: &sdapp.LandingPage{Enabled: true, HealthCheck: &sdapp.HealthCheck{Enabled: true, Path: "/ready"}},
	}
	serviceCache.Add(app)
	svc := serviceCache.Get("uid-revoked")
	if svc == nil || svc.HealthCheckConfig == nil {
		t.Fatalf("expected initial health check config, got %+v", svc)
	}
	cfg := *svc.HealthCheckConfig

	app.LandingPage.HealthCheck = nil
	serviceCache.Add(app)

	requester := &countingProbeRequester{}
	checker := NewHealthChecker(serviceCache, time.Second)
	checker.probeRequester = requester
	checker.running["uid-revoked"] = runningProbe{cancel: func() {}, config: cfg}

	if checker.probeIfCurrent(context.Background(), "uid-revoked", &cfg) {
		t.Fatal("expected revoked config to stop the probe loop")
	}
	if requester.called {
		t.Fatal("expected revoked config not to issue an HTTP request")
	}
	if _, ok := checker.running["uid-revoked"]; ok {
		t.Fatal("expected stale running probe bookkeeping to be cleared")
	}
}

func TestProbeRunnerRejectsProtectedTargets(t *testing.T) {
	for _, rawURL := range []string{
		"http://kubernetes.default:443/healthz",
		"http://redis.team-a:6379/",
		"http://keycloak.keycloak:8080/health",
		"http://app.kube-system:8080/health",
		"http://10.96.0.1:443/healthz",
		"http://app.team-a.example.com:8080/health",
	} {
		t.Run(rawURL, func(t *testing.T) {
			if err := validateProbeRunnerTarget(rawURL); err == nil {
				t.Fatal("expected protected target to be rejected")
			}
		})
	}
}

func TestProbeRunnerAllowsServiceNamespaceTarget(t *testing.T) {
	if err := validateProbeRunnerTarget("http://app-backend.team-a:8080/ready"); err != nil {
		t.Fatalf("expected service.namespace target to be allowed, got %v", err)
	}
}

type countingProbeRequester struct {
	called bool
}

func (r *countingProbeRequester) Probe(context.Context, string, *cache.HealthCheckConfig) probeResult {
	r.called = true
	return probeResult{status: "healthy", checkedAt: time.Now(), message: fmt.Sprintf("called=%v", r.called)}
}
