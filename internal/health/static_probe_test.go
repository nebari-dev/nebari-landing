package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/staticservices"
)

// chartKeycloakApp builds the Keycloak card exactly as the webapi does from the
// chart-rendered static-services fixture. Its probe targets the in-cluster
// Keycloak (http://keycloak.keycloak.svc:8080/realms/nebari), which the probe
// runner's NebariApp rules reject on two counts: the 3-label .svc host and the
// protected keycloak namespace.
func chartKeycloakApp(t *testing.T) *sdapp.App {
	t.Helper()
	data, err := os.ReadFile("../staticservices/testdata/chart-keycloak.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	statics, err := staticservices.Parse(data)
	if err != nil || len(statics) != 1 {
		t.Fatalf("Parse() = %v, %v; want one entry", statics, err)
	}
	return statics[0].App()
}

// dialOnly returns a transport that sends connections for host:port to the
// test server instead, and fails any other dial, so the test proves which
// address the probe used without needing cluster DNS.
func dialOnly(t *testing.T, hostPort string, target *httptest.Server) http.RoundTripper {
	t.Helper()
	targetAddr := target.Listener.Addr().String()
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr != hostPort {
				return nil, fmt.Errorf("unexpected dial to %s", addr)
			}
			return (&net.Dialer{}).DialContext(ctx, network, targetAddr)
		},
	}
}

// countingRunner serves the real probe-runner handler and counts requests.
func countingRunner(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	inner := NewProbeRunnerHandler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestStaticKeycloakProbeIsDirectAndReportsRealResponse(t *testing.T) {
	for _, tt := range []struct {
		code        int
		wantStatus  string
		wantMessage string
	}{
		{http.StatusOK, "healthy", "HTTP 200"},
		{http.StatusServiceUnavailable, "unhealthy", "HTTP 503"},
	} {
		t.Run(tt.wantStatus, func(t *testing.T) {
			var gotPath atomic.Value
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath.Store(r.URL.Path)
				w.WriteHeader(tt.code)
			}))
			defer target.Close()
			runner, runnerHits := countingRunner(t)

			app := chartKeycloakApp(t)
			serviceCache := cache.NewServiceCache()
			serviceCache.Add(app)
			svc := serviceCache.Get(app.UID)
			if svc == nil || svc.HealthCheckConfig == nil {
				t.Fatalf("expected a health check config for the static entry, got %+v", svc)
			}
			cfg := *svc.HealthCheckConfig
			if !cfg.Trusted || cfg.ProbeURL != "http://keycloak.keycloak.svc:8080/realms/nebari" {
				t.Fatalf("config = %+v, want trusted probe of the configured Keycloak URL", cfg)
			}

			checker := NewHealthChecker(serviceCache, time.Second)
			if err := checker.SetProbeRunnerURL(runner.URL); err != nil {
				t.Fatalf("SetProbeRunnerURL: %v", err)
			}
			checker.trustedProbeRequester = directProbeRequester{transport: dialOnly(t, "keycloak.keycloak.svc:8080", target)}

			if !checker.probeIfCurrent(context.Background(), app.UID, &cfg) {
				t.Fatal("expected the probe result to be stored")
			}
			if n := runnerHits.Load(); n != 0 {
				t.Fatalf("probe runner received %d requests; a trusted static probe must not go through it", n)
			}
			if p, _ := gotPath.Load().(string); p != "/realms/nebari" {
				t.Fatalf("target saw path %q, want /realms/nebari", p)
			}
			health := serviceCache.Get(app.UID).Health
			if health == nil || health.Status != tt.wantStatus || health.Message != tt.wantMessage {
				t.Fatalf("health = %+v, want %s/%s", health, tt.wantStatus, tt.wantMessage)
			}
		})
	}
}

func TestNebariAppProbeOfKeycloakIsStillRejected(t *testing.T) {
	// The NebariApp form of the same target never gets a probe config...
	serviceCache := cache.NewServiceCache()
	serviceCache.Add(&sdapp.App{
		UID:                              "uid-kc",
		Name:                             "kc",
		Namespace:                        "team-a",
		Hostname:                         "kc.example.com",
		ServiceName:                      "keycloak",
		ServiceNamespace:                 "keycloak",
		ServicePort:                      8080,
		HealthCheckCrossNamespaceAllowed: true,
		LandingPage: &sdapp.LandingPage{Enabled: true, HealthCheck: &sdapp.HealthCheck{
			Enabled: true, Path: "/realms/nebari",
		}},
	})
	if svc := serviceCache.Get("uid-kc"); svc == nil || svc.HealthCheckConfig != nil {
		t.Fatalf("expected the NebariApp Keycloak health check to be dropped, got %+v", svc)
	}

	// ...and an untrusted config for that URL is refused by the probe runner
	// and reported as unknown, without reaching the target.
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
	}))
	defer target.Close()
	runner, runnerHits := countingRunner(t)

	cfg := cache.HealthCheckConfig{ProbeURL: "http://keycloak.keycloak.svc:8080/realms/nebari", IntervalSeconds: 30, TimeoutSeconds: 1}

	checker := NewHealthChecker(serviceCache, time.Second)
	if err := checker.SetProbeRunnerURL(runner.URL); err != nil {
		t.Fatalf("SetProbeRunnerURL: %v", err)
	}
	checker.trustedProbeRequester = directProbeRequester{transport: dialOnly(t, "keycloak.keycloak.svc:8080", target)}

	got := checker.requesterFor(&cfg).Probe(context.Background(), "uid-untrusted", &cfg)
	if runnerHits.Load() != 1 {
		t.Fatalf("probe runner hits = %d, want 1", runnerHits.Load())
	}
	if targetHits.Load() != 0 {
		t.Fatal("untrusted probe reached the Keycloak target")
	}
	if got.status != "unknown" {
		t.Fatalf("result = %+v, want unknown (runner rejection)", got)
	}
}
