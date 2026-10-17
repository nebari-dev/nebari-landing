package cache

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	defaultHealthCheckIntervalSeconds = 30
	minHealthCheckIntervalSeconds     = 10
	maxHealthCheckIntervalSeconds     = 300

	defaultHealthCheckTimeoutSeconds = 5
	minHealthCheckTimeoutSeconds     = 1
	maxHealthCheckTimeoutSeconds     = 30

	maxHealthCheckPathLength = 2048
)

var protectedHealthCheckNamespaces = map[string]struct{}{
	"argocd":               {},
	"cert-manager":         {},
	"database":             {},
	"databases":            {},
	"db":                   {},
	"envoy-gateway-system": {},
	"kube-node-lease":      {},
	"kube-public":          {},
	"kube-system":          {},
	"keycloak":             {},
	"mariadb":              {},
	"metallb-system":       {},
	"mongo":                {},
	"mongodb":              {},
	"mysql":                {},
	"postgres":             {},
	"postgresql":           {},
	"redis":                {},
	"vault":                {},
}

// protectedHealthCheckServices lists individual Services, as
// "namespace/name", that are protected outside the protected namespaces. The
// API server's Service lives in "default", which cannot be protected as a
// whole.
var protectedHealthCheckServices = map[string]struct{}{
	"default/kubernetes": {},
}

var protectedHealthCheckPorts = map[int]struct{}{
	2181:  {},
	2379:  {},
	2380:  {},
	3306:  {},
	5432:  {},
	6379:  {},
	6443:  {},
	9200:  {},
	9300:  {},
	10250: {},
	10255: {},
	11211: {},
	27017: {},
	27018: {},
	27019: {},
}

// ServiceInfo represents a service that appears on the landing page
type ServiceInfo struct {
	UID               string             `json:"uid"`
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	DisplayName       string             `json:"displayName"`
	Description       string             `json:"description"`
	URL               string             `json:"url"`
	Icon              string             `json:"icon,omitempty"`
	IconLight         string             `json:"iconLight,omitempty"`
	IconDark          string             `json:"iconDark,omitempty"`
	Category          string             `json:"category"`
	Priority          int                `json:"priority"`
	Visibility        string             `json:"visibility"`
	RequiredGroups    []string           `json:"requiredGroups,omitempty"`
	Health            *HealthStatus      `json:"health,omitempty"`
	HealthCheckConfig *HealthCheckConfig `json:"-"` // not serialised; used by the health checker
	// Static marks entries loaded from the webapi's static-services config.
	// Not serialised: it only gates server-side behaviour (access requests).
	Static bool `json:"-"`
}

// IconURL returns a single icon URL for theme-neutral contexts (e.g. notifications).
func (s *ServiceInfo) IconURL() string {
	if s.Icon != "" {
		return s.Icon
	}
	if s.IconLight != "" {
		return s.IconLight
	}
	return s.IconDark
}

// HealthCheckConfig holds the resolved probe settings for a service.
// It is built in Add from the App's health check (spec.landingPage.healthCheck
// for a NebariApp, healthCheck for a static entry) and consumed exclusively by
// the health checker.
type HealthCheckConfig struct {
	// ProbeURL is the full HTTP URL the health checker will GET on each interval.
	// Constructed as http://<service-name>.<namespace>:<port><path>, unless the
	// health check gives a complete URL (static entries).
	ProbeURL        string
	IntervalSeconds int
	TimeoutSeconds  int
	// Trusted marks a probe that comes from operator configuration (a static
	// entry's healthCheck.url in the webapi's chart values) rather than from a
	// NebariApp author. The health checker probes trusted configs directly
	// from the webapi: NebariApp target validation and the probe runner apply
	// only to untrusted configs.
	Trusted bool
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	Status    string     `json:"status"` // healthy, unhealthy, unknown
	LastCheck *time.Time `json:"lastCheck,omitempty"`
	Message   string     `json:"message,omitempty"`
}

// ServiceCache maintains an in-memory cache of services
type ServiceCache struct {
	mu       sync.RWMutex
	services map[string]*ServiceInfo // keyed by UID
	// healthCheckDropReasons remembers, per UID, why a requested health check
	// was dropped, so the reason is logged once per change rather than on
	// every watcher update.
	healthCheckDropReasons map[string]string
	log                    logr.Logger
}

// NewServiceCache creates a new service cache
func NewServiceCache() *ServiceCache {
	return &ServiceCache{
		services:               make(map[string]*ServiceInfo),
		healthCheckDropReasons: make(map[string]string),
		log:                    ctrl.Log.WithName("service-cache"),
	}
}

// Add adds or updates a service in the cache from an internal App domain object.
// If a is nil, has no LandingPage, or has a disabled LandingPage, the UID is
// removed from the cache.
func (c *ServiceCache) Add(a *sdapp.App) {
	if a == nil || a.LandingPage == nil || !a.LandingPage.Enabled {
		if a != nil {
			c.Remove(a.UID)
		}
		return
	}

	lp := a.LandingPage
	priority := 100
	if lp.Priority != 0 {
		priority = lp.Priority
	}
	visibility := "private"
	if lp.Visibility != "" {
		visibility = lp.Visibility
	}
	healthCheckConfig, dropReason := buildHealthCheckConfig(a)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.recordHealthCheckDrop(a, dropReason)

	health := c.preserveHealthStatus(a.UID)
	if c.shouldResetHealthStatus(a.UID, healthCheckConfig) {
		health = &HealthStatus{Status: "unknown"}
	}

	service := &ServiceInfo{
		UID:               a.UID,
		Name:              a.Name,
		Namespace:         a.Namespace,
		DisplayName:       lp.DisplayName,
		Description:       lp.Description,
		URL:               buildURL(a),
		Icon:              lp.Icon,
		IconLight:         lp.IconLight,
		IconDark:          lp.IconDark,
		Category:          lp.Category,
		Priority:          priority,
		Visibility:        visibility,
		RequiredGroups:    lp.RequiredGroups,
		Health:            health,
		HealthCheckConfig: healthCheckConfig,
		Static:            a.Static,
	}

	c.services[a.UID] = service
}

// Remove removes a service from the cache
func (c *ServiceCache) Remove(uid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.services, uid)
	delete(c.healthCheckDropReasons, uid)
}

// recordHealthCheckDrop logs why a requested health check was dropped, once
// per change of reason. The caller must hold c.mu for writing.
func (c *ServiceCache) recordHealthCheckDrop(a *sdapp.App, reason string) {
	previous := c.healthCheckDropReasons[a.UID]
	if reason == previous {
		return
	}
	if reason == "" {
		delete(c.healthCheckDropReasons, a.UID)
		c.log.Info("Health check enabled again", "uid", a.UID, "namespace", a.Namespace, "name", a.Name)
		return
	}
	c.healthCheckDropReasons[a.UID] = reason
	c.log.Info("Health check disabled; the card will show health as unknown",
		"uid", a.UID, "namespace", a.Namespace, "name", a.Name, "reason", reason)
}

// Get retrieves a service by UID
func (c *ServiceCache) Get(uid string) *ServiceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.services[uid]
}

// GetByNamespacedName retrieves a service by namespace and name.
func (c *ServiceCache) GetByNamespacedName(namespace, name string) *ServiceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, svc := range c.services {
		if svc.Namespace == namespace && svc.Name == name {
			return svc
		}
	}
	return nil
}

// GetAll returns all services as a slice
func (c *ServiceCache) GetAll() []*ServiceInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	services := make([]*ServiceInfo, 0, len(c.services))
	for _, service := range c.services {
		services = append(services, service)
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Priority != services[j].Priority {
			return services[i].Priority < services[j].Priority
		}
		return services[i].Name < services[j].Name
	})

	return services
}

// GetCategories returns a unique sorted list of all categories
func (c *ServiceCache) GetCategories() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	categoryMap := make(map[string]bool)
	for _, service := range c.services {
		if service.Category != "" {
			categoryMap[service.Category] = true
		}
	}

	categories := make([]string, 0, len(categoryMap))
	for category := range categoryMap {
		categories = append(categories, category)
	}

	sort.Strings(categories)
	return categories
}

// UpdateHealth updates the health status for a service
func (c *ServiceCache) UpdateHealth(uid string, status *HealthStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if service, exists := c.services[uid]; exists {
		service.Health = status
	}
}

// HealthStatusIfCurrent reports whether uid still has cfg as its active
// health-check configuration and, if so, the service's current health status.
// The lock is held only for the lookup: callers must not hold it across a
// network probe, which can take up to the probe timeout.
func (c *ServiceCache) HealthStatusIfCurrent(uid string, cfg *HealthCheckConfig) (prevStatus string, current bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service := c.services[uid]
	if !healthCheckConfigMatches(service, cfg) {
		return "", false
	}
	if service.Health != nil {
		prevStatus = service.Health.Status
	}
	return prevStatus, true
}

// UpdateHealthIfCurrent stores status for uid only while uid still has cfg as
// its active health-check configuration. The currency check and the write
// happen under the same lock, so a result from a probe whose config was
// replaced or revoked while the request was in flight is discarded instead of
// overwriting the new config's status. It reports whether status was stored.
func (c *ServiceCache) UpdateHealthIfCurrent(uid string, cfg *HealthCheckConfig, status *HealthStatus) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	service := c.services[uid]
	if !healthCheckConfigMatches(service, cfg) {
		return false
	}
	service.Health = status
	return true
}

func healthCheckConfigMatches(service *ServiceInfo, cfg *HealthCheckConfig) bool {
	return service != nil && service.HealthCheckConfig != nil && cfg != nil && *service.HealthCheckConfig == *cfg
}

func (c *ServiceCache) preserveHealthStatus(uid string) *HealthStatus {
	if existing := c.services[uid]; existing != nil && existing.Health != nil {
		return existing.Health
	}
	return &HealthStatus{
		Status: "unknown",
	}
}

func (c *ServiceCache) shouldResetHealthStatus(uid string, cfg *HealthCheckConfig) bool {
	existing := c.services[uid]
	return existing != nil && existing.HealthCheckConfig != nil && cfg == nil
}

func buildURL(a *sdapp.App) string {
	if a.LandingPage != nil && a.LandingPage.ExternalURL != "" {
		return a.LandingPage.ExternalURL
	}
	scheme := "https"
	if !a.TLSEnabled {
		scheme = "http"
	}
	return scheme + "://" + a.Hostname
}

// buildHealthCheckConfig constructs a HealthCheckConfig from the app's
// health check settings. It returns a nil config and an empty reason when
// health checking is disabled or not configured, and a nil config with the
// reason when a requested check is dropped because its target is not allowed.
//
// The target checks apply to NebariApp-sourced input, which any author who can
// create a NebariApp controls. Static entries are operator configuration
// (chart values): their full probe URL is taken as given and the config is
// marked Trusted, so the health checker probes it directly from the webapi.
func buildHealthCheckConfig(a *sdapp.App) (*HealthCheckConfig, string) {
	if a.LandingPage == nil || a.LandingPage.HealthCheck == nil || !a.LandingPage.HealthCheck.Enabled {
		return nil, ""
	}
	hc := a.LandingPage.HealthCheck

	interval := clampHealthCheckSeconds(
		hc.IntervalSeconds,
		defaultHealthCheckIntervalSeconds,
		minHealthCheckIntervalSeconds,
		maxHealthCheckIntervalSeconds,
	)
	timeout := clampHealthCheckSeconds(
		hc.TimeoutSeconds,
		defaultHealthCheckTimeoutSeconds,
		minHealthCheckTimeoutSeconds,
		maxHealthCheckTimeoutSeconds,
	)
	if timeout > interval {
		timeout = interval
	}
	if a.Static {
		if hc.URL == "" {
			return nil, "static healthCheck has no url"
		}
		return &HealthCheckConfig{
			ProbeURL:        hc.URL,
			IntervalSeconds: interval,
			TimeoutSeconds:  timeout,
			Trusted:         true,
		}, ""
	}
	// A full probe URL is operator-only: a NebariApp must probe its declared
	// Service, so the address is built from the Service reference below.
	if hc.URL != "" {
		return nil, "healthCheck.url is accepted only from static configuration"
	}
	path, ok := normalizeHealthCheckPath(hc.Path)
	if !ok {
		return nil, fmt.Sprintf("healthCheck.path %q is not a valid absolute request path", hc.Path)
	}
	// Probe the Kubernetes service directly using in-cluster DNS so the health
	// check bypasses the ingress/gateway and always uses HTTP regardless of
	// whether TLS is configured for external access.
	serviceName := a.ServiceName
	if serviceName == "" {
		serviceName = a.Name
	}
	if len(validation.IsDNS1035Label(serviceName)) != 0 {
		return nil, fmt.Sprintf("service name %q is not a valid DNS-1035 label", serviceName)
	}
	port := a.ServicePort
	if port == 0 {
		port = 80
	}
	// healthCheck.port overrides the Service port, as the NebariApp CRD
	// documents, for backends that serve health on a separate management port
	// (e.g. Keycloak X serves /health/ready on 9000, not 8080). The target is
	// still the declared Service; protected ports are rejected below.
	if hc.Port > 0 {
		port = hc.Port
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Sprintf("health check port %d is outside 1-65535", port)
	}
	// Use ServiceNamespace (spec.service.namespace) only after enforcing the
	// cross-namespace consent decision made by the watcher.
	serviceNamespace := a.ServiceNamespace
	if serviceNamespace == "" {
		serviceNamespace = a.Namespace
	}
	if len(validation.IsDNS1123Label(serviceNamespace)) != 0 {
		return nil, fmt.Sprintf("service namespace %q is not a valid DNS-1123 label", serviceNamespace)
	}
	if serviceNamespace != a.Namespace && !a.HealthCheckCrossNamespaceAllowed {
		return nil, fmt.Sprintf(
			"target Service %s/%s is in another namespace and no ReferenceGrant in %q permits NebariApps from %q to reference it",
			serviceNamespace, serviceName, serviceNamespace, a.Namespace)
	}
	if reason := HealthCheckTargetProtection(serviceNamespace, serviceName, port); reason != "" {
		return nil, fmt.Sprintf("target %s.%s:%d is protected: %s", serviceName, serviceNamespace, port, reason)
	}
	return &HealthCheckConfig{
		ProbeURL:        fmt.Sprintf("http://%s.%s:%d%s", serviceName, serviceNamespace, port, path),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
	}, ""
}

func normalizeHealthCheckPath(path string) (string, bool) {
	if path == "" {
		return "/", true
	}
	if len(path) > maxHealthCheckPathLength {
		return "", false
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "", false
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	parsed, err := url.ParseRequestURI(path)
	if err != nil {
		return "", false
	}
	if parsed.Scheme != "" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
		return "", false
	}
	return path, true
}

func clampHealthCheckSeconds(value, defaultValue, minValue, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

// HealthCheckTargetProtected reports whether a health probe target is in a
// namespace/service/port class that must not be reachable from NebariApp-owned
// health checks.
func HealthCheckTargetProtected(namespace, serviceName string, port int) bool {
	return HealthCheckTargetProtection(namespace, serviceName, port) != ""
}

// HealthCheckTargetProtection returns why a health probe target is protected,
// or "" when it is not. Protection is by namespace (identity, GitOps,
// control-plane, and database namespaces, plus any "*-system" namespace), by
// explicitly listed Service, and by well-known non-HTTP infrastructure port.
// Service names are not pattern-matched: a token heuristic (any name
// containing "kube", "dns", "redis", ...) dropped legitimate checks such as
// kube-prometheus-stack-grafana.
func HealthCheckTargetProtection(namespace, serviceName string, port int) string {
	if _, ok := protectedHealthCheckNamespaces[namespace]; ok {
		return fmt.Sprintf("namespace %q is protected", namespace)
	}
	if strings.HasSuffix(namespace, "-system") {
		return fmt.Sprintf("namespace %q is a system namespace", namespace)
	}
	if _, ok := protectedHealthCheckServices[namespace+"/"+serviceName]; ok {
		return fmt.Sprintf("Service %s/%s is protected", namespace, serviceName)
	}
	if _, ok := protectedHealthCheckPorts[port]; ok {
		return fmt.Sprintf("port %d is protected", port)
	}
	return ""
}
