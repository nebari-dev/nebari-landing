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

// blockingProbeRequester parks every probe until release is closed, so a test
// can act on the cache while a probe is in flight.
type blockingProbeRequester struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingProbeRequester() *blockingProbeRequester {
	return &blockingProbeRequester{started: make(chan struct{}, 1), release: make(chan struct{})}
}

func (r *blockingProbeRequester) Probe(context.Context, string, *cache.HealthCheckConfig) probeResult {
	r.started <- struct{}{}
	<-r.release
	return probeResult{status: "healthy", checkedAt: time.Now(), message: "HTTP 200"}
}

func healthCheckedApp(uid, path string) *sdapp.App {
	return &sdapp.App{
		UID:         uid,
		Name:        "app",
		Namespace:   "team-a",
		Hostname:    "app.example.com",
		TLSEnabled:  true,
		ServiceName: "app-backend",
		ServicePort: 8080,
		LandingPage: &sdapp.LandingPage{Enabled: true, HealthCheck: &sdapp.HealthCheck{Enabled: true, Path: path}},
	}
}

// startBlockedProbe starts probeIfCurrent for uid in the background and waits
// until the probe request is in flight.
func startBlockedProbe(t *testing.T, checker *HealthChecker, requester *blockingProbeRequester, uid string, cfg *cache.HealthCheckConfig) <-chan bool {
	t.Helper()
	done := make(chan bool, 1)
	go func() { done <- checker.probeIfCurrent(context.Background(), uid, cfg) }()
	select {
	case <-requester.started:
	case <-time.After(5 * time.Second):
		t.Fatal("probe never started")
	}
	return done
}

// TestProbeIfCurrentDoesNotHoldCacheLockDuringProbe guards against holding the
// cache lock across the HTTP probe: a slow or dropped target must not stall
// watcher writes (Add) or API reads (GetAll) for unrelated services.
func TestProbeIfCurrentDoesNotHoldCacheLockDuringProbe(t *testing.T) {
	serviceCache := cache.NewServiceCache()
	serviceCache.Add(healthCheckedApp("uid-slow", "/ready"))
	cfg := *serviceCache.Get("uid-slow").HealthCheckConfig

	requester := newBlockingProbeRequester()
	checker := NewHealthChecker(serviceCache, time.Second)
	checker.probeRequester = requester
	probeDone := startBlockedProbe(t, checker, requester, "uid-slow", &cfg)
	defer close(requester.release)

	// Add takes the write lock; if the probe held the read lock this would
	// block until release, and GetAll would queue behind it.
	cacheDone := make(chan struct{})
	go func() {
		defer close(cacheDone)
		serviceCache.Add(makeOtherApp("uid-other"))
		_ = serviceCache.GetAll()
	}()
	select {
	case <-cacheDone:
	case <-time.After(2 * time.Second):
		t.Fatal("cache.Add/GetAll blocked while a health probe was in flight")
	}

	requester.release <- struct{}{}
	if !<-probeDone {
		t.Fatal("expected probe with unchanged config to report current")
	}
	if got := serviceCache.Get("uid-slow").Health.Status; got != "healthy" {
		t.Fatalf("health status = %q, want healthy", got)
	}
}

// TestProbeIfCurrentDiscardsResultWhenConfigChangesMidProbe checks that, with
// the lock released during the probe, a result for a config that was replaced
// while the request was in flight is dropped rather than written over the new
// config's status.
func TestProbeIfCurrentDiscardsResultWhenConfigChangesMidProbe(t *testing.T) {
	serviceCache := cache.NewServiceCache()
	serviceCache.Add(healthCheckedApp("uid-changed", "/ready"))
	cfg := *serviceCache.Get("uid-changed").HealthCheckConfig

	requester := newBlockingProbeRequester()
	checker := NewHealthChecker(serviceCache, time.Second)
	checker.probeRequester = requester
	checker.running["uid-changed"] = runningProbe{cancel: func() {}, config: cfg}
	probeDone := startBlockedProbe(t, checker, requester, "uid-changed", &cfg)

	addDone := make(chan struct{})
	go func() {
		defer close(addDone)
		serviceCache.Add(healthCheckedApp("uid-changed", "/healthz"))
	}()
	select {
	case <-addDone:
	case <-time.After(2 * time.Second):
		close(requester.release)
		t.Fatal("cache.Add blocked while a health probe was in flight")
	}
	close(requester.release)

	if <-probeDone {
		t.Fatal("expected probe for a replaced config to stop")
	}
	if got := serviceCache.Get("uid-changed").Health.Status; got != "unknown" {
		t.Fatalf("health status = %q, want unknown (stale result discarded)", got)
	}
	if _, ok := checker.running["uid-changed"]; ok {
		t.Fatal("expected stale running probe bookkeeping to be cleared")
	}
}

func makeOtherApp(uid string) *sdapp.App {
	return &sdapp.App{
		UID:         uid,
		Name:        "other",
		Namespace:   "team-b",
		Hostname:    "other.example.com",
		LandingPage: &sdapp.LandingPage{Enabled: true},
	}
}

func TestRemoteProbeRequesterReportsRunnerFailuresAsUnknown(t *testing.T) {
	cfg := &cache.HealthCheckConfig{ProbeURL: "http://app-backend.team-a:8080/ready", IntervalSeconds: 30, TimeoutSeconds: 1}

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		wantStatus  string
		wantMessage string
	}{
		{
			name: "runner rejects protected target",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "probe target is protected", http.StatusForbidden)
			},
			wantStatus:  "unknown",
			wantMessage: "probe runner rejected target: probe target is protected",
		},
		{
			name: "runner internal error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			wantStatus:  "unknown",
			wantMessage: "probe runner HTTP 500: boom",
		},
		{
			name: "runner measured an unhealthy target",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"unhealthy","message":"HTTP 503"}`))
			},
			wantStatus:  "unhealthy",
			wantMessage: "HTTP 503",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()
			requester, err := newRemoteProbeRequester(server.URL)
			if err != nil {
				t.Fatalf("newRemoteProbeRequester: %v", err)
			}
			got := requester.Probe(context.Background(), "uid", cfg)
			if got.status != tt.wantStatus || got.message != tt.wantMessage {
				t.Fatalf("Probe() = %q/%q, want %q/%q", got.status, got.message, tt.wantStatus, tt.wantMessage)
			}
		})
	}
}

func TestRemoteProbeRequesterReportsUnreachableRunnerAsUnknown(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	runnerURL := server.URL
	server.Close()

	requester, err := newRemoteProbeRequester(runnerURL)
	if err != nil {
		t.Fatalf("newRemoteProbeRequester: %v", err)
	}
	got := requester.Probe(context.Background(), "uid", &cache.HealthCheckConfig{ProbeURL: "http://app-backend.team-a:8080/ready", TimeoutSeconds: 1})
	if got.status != "unknown" {
		t.Fatalf("status = %q (%s), want unknown", got.status, got.message)
	}
}
