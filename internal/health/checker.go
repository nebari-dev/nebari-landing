package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/notifications"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("health-checker")

// Publisher is an optional sink that receives a notification whenever a
// service's health status changes. *websocket.Hub satisfies this interface.
type Publisher interface {
	Publish(eventType string, service *cache.ServiceInfo)
}

// HealthChecker performs periodic HTTP health checks on services registered
// in the ServiceCache. Each service that has a HealthCheckConfig is probed
// independently on its own interval using a lightweight goroutine-per-service
// model; goroutines exit when ctx is cancelled or the service is removed.
type HealthChecker struct {
	cache      *cache.ServiceCache
	interval   time.Duration        // fallback global interval when service doesn't specify one
	publisher  Publisher            // optional; may be nil
	notifStore *notifications.Store // optional; when set, "back online" notifications are posted
	// running maps UID → (cancel func, current ProbeURL).
	// The ProbeURL is stored so reconcile can detect config changes and restart
	// the probe goroutine when a NebariApp's healthCheck spec is updated.
	mu      sync.Mutex
	running map[string]runningProbe
}

type runningProbe struct {
	cancel   context.CancelFunc
	probeURL string // the URL this goroutine is currently configured to probe
}

// NewHealthChecker creates a new health checker.
// interval is the fallback polling interval used when a service's
// HealthCheckConfig doesn't specify one.
func NewHealthChecker(serviceCache *cache.ServiceCache, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		cache:    serviceCache,
		interval: interval,
		running:  make(map[string]runningProbe),
	}
}

// SetPublisher attaches an event publisher that is notified whenever a
// service's health status transitions (e.g. unknown → healthy).
func (h *HealthChecker) SetPublisher(p Publisher) {
	h.publisher = p
}

// SetNotificationStore attaches a notification store. When set, a platform
// notification is automatically posted whenever a service transitions from
// "unhealthy" back to "healthy" (i.e. it is "back online").
func (h *HealthChecker) SetNotificationStore(s *notifications.Store) {
	h.notifStore = s
}

// postRecoveryNotif posts a "back online" notification for the given service UID.
func (h *HealthChecker) postRecoveryNotif(uid string) {
	if h.notifStore == nil {
		return
	}
	svc := h.cache.Get(uid)
	if svc == nil {
		return
	}
	name := svc.DisplayName
	if name == "" {
		name = svc.Name
	}
	if _, err := h.notifStore.Create(
		svc.Icon,
		fmt.Sprintf("%s is back online!", name),
		fmt.Sprintf("%s is back online! Service is ready to use.", name),
	); err != nil {
		log.Error(err, "Failed to post recovery notification", "uid", uid, "name", name)
	}
}

// Start starts the health checker. It periodically reconciles the set of
// active probe goroutines against the service cache and launches new ones for
// services that have a HealthCheckConfig. Goroutines for removed services are
// cancelled automatically on the next reconcile tick.
func (h *HealthChecker) Start(ctx context.Context) {
	log.Info("Health checker started", "fallback-interval", h.interval)

	// Reconcile immediately, then on every interval tick.
	h.reconcile(ctx)
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.reconcile(ctx)
		case <-ctx.Done():
			h.stopAll()
			log.Info("Health checker stopped")
			return
		}
	}
}

// reconcile syncs the set of running probe goroutines with the current
// service cache contents.
func (h *HealthChecker) reconcile(ctx context.Context) {
	services := h.cache.GetAll()
	activeUIDs := make(map[string]bool, len(services))

	checkable := 0
	for _, svc := range services {
		if svc.HealthCheckConfig == nil {
			continue
		}
		checkable++
		activeUIDs[svc.UID] = true

		h.mu.Lock()
		rp, running := h.running[svc.UID]
		// Restart the goroutine if the probe URL changed (NebariApp was updated).
		if running && rp.probeURL != svc.HealthCheckConfig.ProbeURL {
			log.Info("Probe URL changed — restarting probe goroutine",
				"uid", svc.UID, "old", rp.probeURL, "new", svc.HealthCheckConfig.ProbeURL)
			rp.cancel()
			running = false
			delete(h.running, svc.UID)
		}
		if !running {
			probeCtx, cancel := context.WithCancel(ctx)
			h.running[svc.UID] = runningProbe{cancel: cancel, probeURL: svc.HealthCheckConfig.ProbeURL}
			log.Info("Starting probe goroutine", "uid", svc.UID, "name", svc.DisplayName, "url", svc.HealthCheckConfig.ProbeURL)
			go h.probeLoop(probeCtx, svc.UID, svc.HealthCheckConfig)
		}
		h.mu.Unlock()
	}

	log.Info("Reconcile complete", "total", len(services), "checkable", checkable, "goroutines", len(h.running))

	// Stop probes for services that have been removed from the cache.
	h.mu.Lock()
	for uid, rp := range h.running {
		if !activeUIDs[uid] {
			rp.cancel()
			delete(h.running, uid)
		}
	}
	h.mu.Unlock()
}

// stopAll cancels every running probe goroutine.
func (h *HealthChecker) stopAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for uid, rp := range h.running {
		rp.cancel()
		delete(h.running, uid)
	}
}

// probeLoop runs a single service's health check on its configured interval
// until ctx is cancelled.
func (h *HealthChecker) probeLoop(ctx context.Context, uid string, cfg *cache.HealthCheckConfig) {
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = h.interval
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	log.Info("Probe loop started", "uid", uid, "url", cfg.ProbeURL, "interval", interval, "timeout", timeout)

	client := &http.Client{
		Timeout: timeout,
		// Don't follow redirects — a redirect means the service is up but
		// redirecting (e.g. HTTP→HTTPS). Treat any response as "reachable".
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Probe immediately, then repeat on interval.
	h.probe(ctx, uid, cfg.ProbeURL, client)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.probe(ctx, uid, cfg.ProbeURL, client)
		case <-ctx.Done():
			log.V(1).Info("Probe loop stopped", "uid", uid)
			return
		}
	}
}

// probe performs a single HTTP GET against probeURL and updates the service
// cache. It publishes a "modified" event when the status transitions.
func (h *HealthChecker) probe(ctx context.Context, uid, probeURL string, client *http.Client) {
	// Snapshot the current health status before probing so we can detect
	// an unhealthy → healthy transition after the result is in.
	prevStatus := ""
	if svc := h.cache.Get(uid); svc != nil && svc.Health != nil {
		prevStatus = svc.Health.Status
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		log.Info("Health probe request error", "uid", uid, "url", probeURL, "err", err)
		h.setStatus(uid, "unknown", fmt.Sprintf("failed to build request: %v", err))
		return
	}

	now := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Info("Health probe failed", "uid", uid, "url", probeURL, "err", err)
		h.setStatus(uid, "unhealthy", fmt.Sprintf("probe error: %v", err))
		return
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.V(1).Info("Health probe: failed to close response body", "uid", uid, "err", closeErr)
		}
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		log.Info("Health probe ok", "uid", uid, "url", probeURL, "status", resp.StatusCode)
		h.cache.UpdateHealth(uid, &cache.HealthStatus{
			Status:    "healthy",
			LastCheck: &now,
			Message:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		})
		// Post a "back online" notification when recovering from unhealthy.
		if prevStatus == "unhealthy" {
			h.postRecoveryNotif(uid)
		}
	} else {
		log.Info("Health probe unhealthy", "uid", uid, "url", probeURL, "status", resp.StatusCode)
		h.setStatus(uid, "unhealthy", fmt.Sprintf("HTTP %d", resp.StatusCode))
	}

	h.publishIfChanged(uid)
}

// setStatus writes a HealthStatus with the given status string and message.
func (h *HealthChecker) setStatus(uid, status, message string) {
	now := time.Now()
	h.cache.UpdateHealth(uid, &cache.HealthStatus{
		Status:    status,
		LastCheck: &now,
		Message:   message,
	})
	h.publishIfChanged(uid)
}

// publishIfChanged emits a "modified" WebSocket event for the service. The
// publisher decides whether downstream clients actually see a diff.
func (h *HealthChecker) publishIfChanged(uid string) {
	if h.publisher == nil {
		return
	}
	svc := h.cache.Get(uid)
	if svc != nil {
		h.publisher.Publish("modified", svc)
	}
}
