package cache

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
	"k8s.io/apimachinery/pkg/util/validation"
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

var protectedHealthCheckServiceTokens = map[string]struct{}{
	"coredns":    {},
	"dns":        {},
	"etcd":       {},
	"keycloak":   {},
	"kube":       {},
	"kubernetes": {},
	"mariadb":    {},
	"metadata":   {},
	"mongo":      {},
	"mongodb":    {},
	"mysql":      {},
	"postgres":   {},
	"postgresql": {},
	"redis":      {},
	"vault":      {},
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
// It is populated by the watcher from spec.landingPage.healthCheck in the
// NebariApp CRD and consumed exclusively by the health checker.
type HealthCheckConfig struct {
	// ProbeURL is the full HTTP URL the health checker will GET on each interval.
	// Constructed as http://<service-name>.<namespace>:<port><path>.
	ProbeURL        string
	IntervalSeconds int
	TimeoutSeconds  int
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
}

// NewServiceCache creates a new service cache
func NewServiceCache() *ServiceCache {
	return &ServiceCache{
		services: make(map[string]*ServiceInfo),
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
	healthCheckConfig := buildHealthCheckConfig(a)

	c.mu.Lock()
	defer c.mu.Unlock()

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
	}

	c.services[a.UID] = service
}

// Remove removes a service from the cache
func (c *ServiceCache) Remove(uid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.services, uid)
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

// WithCurrentHealthCheck runs fn only while uid still has cfg as its active
// health-check configuration. The read lock is held for fn so writers that
// remove or replace cfg cannot interleave between the freshness check and the
// protected operation.
func (c *ServiceCache) WithCurrentHealthCheck(uid string, cfg *HealthCheckConfig, fn func(prevStatus string)) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service := c.services[uid]
	if service == nil || service.HealthCheckConfig == nil || cfg == nil || *service.HealthCheckConfig != *cfg {
		return false
	}
	prevStatus := ""
	if service.Health != nil {
		prevStatus = service.Health.Status
	}
	fn(prevStatus)
	return true
}

// HealthCheckConfigCurrent reports whether uid still has cfg as its active
// health-check configuration.
func (c *ServiceCache) HealthCheckConfigCurrent(uid string, cfg *HealthCheckConfig) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	service := c.services[uid]
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
// health check settings. Returns nil when health checking is disabled or
// not configured.
func buildHealthCheckConfig(a *sdapp.App) *HealthCheckConfig {
	if a.LandingPage == nil || a.LandingPage.HealthCheck == nil || !a.LandingPage.HealthCheck.Enabled {
		return nil
	}
	hc := a.LandingPage.HealthCheck
	path, ok := normalizeHealthCheckPath(hc.Path)
	if !ok {
		return nil
	}

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

	// Probe the Kubernetes service directly using in-cluster DNS so the health
	// check bypasses the ingress/gateway and always uses HTTP regardless of
	// whether TLS is configured for external access.
	serviceName := a.ServiceName
	if serviceName == "" {
		serviceName = a.Name
	}
	if len(validation.IsDNS1035Label(serviceName)) != 0 {
		return nil
	}
	servicePort := a.ServicePort
	if servicePort == 0 {
		servicePort = 80
	}
	// healthCheck.port is not allowed to widen the target beyond the declared
	// backend Service port; otherwise a NebariApp author could probe arbitrary
	// ports on an otherwise legitimate Service.
	if hc.Port > 0 && hc.Port != servicePort {
		return nil
	}
	if servicePort < 1 || servicePort > 65535 {
		return nil
	}
	// Use ServiceNamespace (spec.service.namespace) only after enforcing the
	// cross-namespace consent decision made by the watcher.
	serviceNamespace := a.ServiceNamespace
	if serviceNamespace == "" {
		serviceNamespace = a.Namespace
	}
	if len(validation.IsDNS1123Label(serviceNamespace)) != 0 {
		return nil
	}
	if serviceNamespace != a.Namespace && !a.HealthCheckCrossNamespaceAllowed {
		return nil
	}
	if HealthCheckTargetProtected(serviceNamespace, serviceName, servicePort) {
		return nil
	}
	return &HealthCheckConfig{
		ProbeURL:        fmt.Sprintf("http://%s.%s:%d%s", serviceName, serviceNamespace, servicePort, path),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
	}
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
	if _, ok := protectedHealthCheckNamespaces[namespace]; ok {
		return true
	}
	if strings.HasSuffix(namespace, "-system") {
		return true
	}
	if _, ok := protectedHealthCheckPorts[port]; ok {
		return true
	}
	for _, token := range strings.Split(serviceName, "-") {
		if _, ok := protectedHealthCheckServiceTokens[token]; ok {
			return true
		}
	}
	return false
}
