package health

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// NotificationPublisher broadcasts a new notification to connected WebSocket
// clients. *websocket.Hub satisfies this interface.
type NotificationPublisher interface {
	PublishNotification(n *notifications.Notification)
}

// HealthChecker performs periodic HTTP health checks on services registered
// in the ServiceCache. Each service that has a HealthCheckConfig is probed
// independently on its own interval using a lightweight goroutine-per-service
// model; goroutines exit when ctx is cancelled or the service is removed.
type HealthChecker struct {
	cache          *cache.ServiceCache
	interval       time.Duration         // fallback global interval when service doesn't specify one
	publisher      Publisher             // optional; may be nil
	notifPublisher NotificationPublisher // optional; may be nil
	notifStore     *notifications.Store  // optional; when set, "back online" notifications are posted
	// probeRequester runs untrusted (NebariApp) probes: through the isolated
	// probe runner when one is configured, otherwise directly.
	probeRequester probeRequester
	// trustedProbeRequester runs trusted (operator-configured static entry)
	// probes, always directly from the webapi. The probe runner's NebariApp
	// target rules would reject them (e.g. an in-cluster Keycloak URL), and
	// the operator, not a NebariApp author, chose the target.
	trustedProbeRequester probeRequester
	// running maps UID → (cancel func, current config). The config is stored so
	// reconcile can detect changes and restart the probe goroutine when a
	// NebariApp's healthCheck spec is updated.
	mu      sync.Mutex
	running map[string]runningProbe
}

type runningProbe struct {
	cancel context.CancelFunc
	config cache.HealthCheckConfig
}

type probeRequester interface {
	Probe(ctx context.Context, uid string, cfg *cache.HealthCheckConfig) probeResult
}

// directProbeRequester probes from the webapi process itself. transport is
// nil in production (http.DefaultTransport); tests set it to redirect dials.
type directProbeRequester struct {
	transport http.RoundTripper
}

// NewHealthChecker creates a new health checker.
// interval is the fallback polling interval used when a service's
// HealthCheckConfig doesn't specify one.
func NewHealthChecker(serviceCache *cache.ServiceCache, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		cache:                 serviceCache,
		interval:              interval,
		probeRequester:        directProbeRequester{},
		trustedProbeRequester: directProbeRequester{},
		running:               make(map[string]runningProbe),
	}
}

// SetProbeRunnerURL delegates untrusted (NebariApp) health probes to a
// separate probe-runner HTTP service; trusted static-entry probes stay direct. Keeping the runner in its own pod lets Kubernetes NetworkPolicy give
// probe traffic a narrower egress policy than the main webapi pod needs for
// Redis, Keycloak, and Kubernetes API access.
func (h *HealthChecker) SetProbeRunnerURL(rawURL string) error {
	requester, err := newRemoteProbeRequester(rawURL)
	if err != nil {
		return err
	}
	h.probeRequester = requester
	return nil
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

// SetNotificationPublisher attaches a publisher that broadcasts new
// notifications to connected WebSocket clients (e.g. *websocket.Hub).
func (h *HealthChecker) SetNotificationPublisher(p NotificationPublisher) {
	h.notifPublisher = p
}

// postRecoveryNotif posts a "back online" notification for the given service UID
// and broadcasts it to connected WebSocket clients.
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
	// Tag with the source-service metadata so per-caller filtering can gate
	// delivery: a private service's recovery notification must only reach
	// callers whose groups satisfy svc.RequiredGroups.
	n, err := h.notifStore.CreateDraft(notifications.Draft{
		Image:          svc.IconURL(),
		Title:          fmt.Sprintf("%s is back online!", name),
		Message:        fmt.Sprintf("%s is back online! Service is ready to use.", name),
		ServiceUID:     svc.UID,
		Visibility:     svc.Visibility,
		RequiredGroups: svc.RequiredGroups,
	})
	if err != nil {
		log.Error(err, "Failed to post recovery notification", "uid", uid, "name", name)
		return
	}
	if h.notifPublisher != nil {
		h.notifPublisher.PublishNotification(n)
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
		// Restart the goroutine if the probe config changed (NebariApp was updated).
		if running && rp.config != *svc.HealthCheckConfig {
			log.Info("Probe config changed; restarting probe goroutine",
				"uid", svc.UID, "oldURL", rp.config.ProbeURL, "newURL", svc.HealthCheckConfig.ProbeURL)
			rp.cancel()
			running = false
			delete(h.running, svc.UID)
		}
		if !running {
			probeCtx, cancel := context.WithCancel(ctx)
			h.running[svc.UID] = runningProbe{cancel: cancel, config: *svc.HealthCheckConfig}
			log.Info("Starting probe goroutine", "uid", svc.UID, "name", svc.DisplayName, "url", svc.HealthCheckConfig.ProbeURL)
			go h.probeLoop(probeCtx, svc.UID, svc.HealthCheckConfig)
		}
		h.mu.Unlock()
	}

	// Stop probes for services that have been removed from the cache.
	h.mu.Lock()
	for uid, rp := range h.running {
		if !activeUIDs[uid] {
			rp.cancel()
			delete(h.running, uid)
		}
	}
	runningCount := len(h.running)
	h.mu.Unlock()

	log.Info("Reconcile complete", "total", len(services), "checkable", checkable, "goroutines", runningCount)
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

	// Probe immediately, then repeat on interval.
	if !h.probeIfCurrent(ctx, uid, cfg) {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !h.probeIfCurrent(ctx, uid, cfg) {
				return
			}
		case <-ctx.Done():
			log.V(1).Info("Probe loop stopped", "uid", uid)
			return
		}
	}
}

func (r directProbeRequester) Probe(ctx context.Context, uid string, cfg *cache.HealthCheckConfig) probeResult {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	client := newProbeHTTPClient(timeout)
	if r.transport != nil {
		client.Transport = r.transport
	}
	return performProbeRequest(ctx, uid, cfg.ProbeURL, client)
}

// requesterFor picks how cfg is probed: trusted operator config directly,
// NebariApp config through the (possibly remote) untrusted requester.
func (h *HealthChecker) requesterFor(cfg *cache.HealthCheckConfig) probeRequester {
	if cfg.Trusted {
		return h.trustedProbeRequester
	}
	return h.probeRequester
}

func newProbeHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		// Don't follow redirects. A redirect moves the probe to a target the
		// NebariApp did not declare, so the response is evaluated as-is.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// probeIfCurrent runs one probe for cfg and stores the result, unless cfg is
// no longer the service's active config. The cache lock is not held during the
// probe: a probe can take the full configured timeout (plus the hop to the
// probe runner), and holding even a read lock that long queues every cache
// writer and, behind them, every reader (watcher updates, GET /services).
// Instead the config is checked before the probe and re-checked atomically
// with the write, so a result for a config that changed mid-flight is dropped.
func (h *HealthChecker) probeIfCurrent(ctx context.Context, uid string, cfg *cache.HealthCheckConfig) bool {
	prevStatus, current := h.cache.HealthStatusIfCurrent(uid, cfg)
	if !current {
		log.Info("Probe config no longer current; stopping probe loop", "uid", uid, "url", cfg.ProbeURL)
		h.forgetRunningProbe(uid, cfg)
		return false
	}
	result := h.requesterFor(cfg).Probe(ctx, uid, cfg)
	if !h.cache.UpdateHealthIfCurrent(uid, cfg, result.healthStatus()) {
		log.Info("Probe config changed while request was in flight; discarding stale result", "uid", uid, "url", cfg.ProbeURL)
		h.forgetRunningProbe(uid, cfg)
		return false
	}
	h.afterHealthUpdate(uid, prevStatus, result.status)
	return true
}

func (h *HealthChecker) forgetRunningProbe(uid string, cfg *cache.HealthCheckConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if rp, ok := h.running[uid]; ok && rp.config == *cfg {
		rp.cancel()
		delete(h.running, uid)
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

	h.applyProbeResult(uid, prevStatus, performProbeRequest(ctx, uid, probeURL, client))
}

type probeResult struct {
	status    string
	message   string
	checkedAt time.Time
}

func (r probeResult) healthStatus() *cache.HealthStatus {
	checkedAt := r.checkedAt
	return &cache.HealthStatus{
		Status:    r.status,
		LastCheck: &checkedAt,
		Message:   r.message,
	}
}

func performProbeRequest(ctx context.Context, uid, probeURL string, client *http.Client) probeResult {
	now := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		log.Info("Health probe request error", "uid", uid, "url", probeURL, "err", err)
		return probeResult{status: "unknown", checkedAt: now, message: fmt.Sprintf("failed to build request: %v", err)}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Info("Health probe failed", "uid", uid, "url", probeURL, "err", err)
		return probeResult{status: "unhealthy", checkedAt: now, message: fmt.Sprintf("probe error: %v", err)}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.V(1).Info("Health probe: failed to close response body", "uid", uid, "err", closeErr)
		}
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info("Health probe ok", "uid", uid, "url", probeURL, "status", resp.StatusCode)
		return probeResult{status: "healthy", checkedAt: now, message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	log.Info("Health probe unhealthy", "uid", uid, "url", probeURL, "status", resp.StatusCode)
	return probeResult{status: "unhealthy", checkedAt: now, message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

type remoteProbeRequester struct {
	endpoint string
	client   *http.Client
}

type remoteProbeRequest struct {
	ProbeURL       string `json:"probeUrl"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

type remoteProbeResponse struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checkedAt"`
}

func newRemoteProbeRequester(rawURL string) (*remoteProbeRequester, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid health probe runner URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("health probe runner URL must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("health probe runner URL must include a host")
	}
	return &remoteProbeRequester{
		endpoint: parsed.ResolveReference(&url.URL{Path: "/probe"}).String(),
		client: &http.Client{
			Timeout: 35 * time.Second,
		},
	}, nil
}

func (r *remoteProbeRequester) Probe(ctx context.Context, uid string, cfg *cache.HealthCheckConfig) probeResult {
	now := time.Now()
	payload, err := json.Marshal(remoteProbeRequest{
		ProbeURL:       cfg.ProbeURL,
		TimeoutSeconds: cfg.TimeoutSeconds,
	})
	if err != nil {
		return probeResult{status: "unknown", checkedAt: now, message: fmt.Sprintf("failed to build probe request: %v", err)}
	}

	timeout := time.Duration(cfg.TimeoutSeconds)*time.Second + time.Second
	if timeout <= time.Second {
		timeout = 6 * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, r.endpoint, bytes.NewReader(payload))
	if err != nil {
		return probeResult{status: "unknown", checkedAt: now, message: fmt.Sprintf("failed to build probe runner request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	// Failures between the webapi and the runner, and targets the runner
	// refuses, say nothing about the target's health: report them as
	// "unknown" with the reason so a policy denial or a runner outage does
	// not look like the service being down. Only a result the runner actually
	// measured can be "unhealthy".
	resp, err := r.client.Do(req)
	if err != nil {
		return probeResult{status: "unknown", checkedAt: now, message: fmt.Sprintf("probe runner unavailable: %v", err)}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.V(1).Info("Health probe runner: failed to close response body", "uid", uid, "err", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		reason := string(bytes.TrimSpace(body))
		message := fmt.Sprintf("probe runner HTTP %d: %s", resp.StatusCode, reason)
		if resp.StatusCode == http.StatusForbidden {
			message = fmt.Sprintf("probe runner rejected target: %s", reason)
		}
		log.Info("Health probe not run by probe runner", "uid", uid, "url", cfg.ProbeURL, "status", resp.StatusCode, "reason", reason)
		return probeResult{status: "unknown", checkedAt: now, message: message}
	}

	var out remoteProbeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return probeResult{status: "unknown", checkedAt: now, message: fmt.Sprintf("failed to decode probe runner response: %v", err)}
	}
	if out.CheckedAt.IsZero() {
		out.CheckedAt = now
	}
	if out.Status == "" {
		out.Status = "unknown"
	}
	return probeResult{status: out.Status, checkedAt: out.CheckedAt, message: out.Message}
}

func (h *HealthChecker) applyProbeResult(uid, prevStatus string, result probeResult) {
	h.cache.UpdateHealth(uid, result.healthStatus())
	h.afterHealthUpdate(uid, prevStatus, result.status)
}

// afterHealthUpdate posts the "back online" notification when a service
// recovers from unhealthy and publishes the updated service.
func (h *HealthChecker) afterHealthUpdate(uid, prevStatus, status string) {
	if status == "healthy" && prevStatus == "unhealthy" {
		h.postRecoveryNotif(uid)
	}
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
