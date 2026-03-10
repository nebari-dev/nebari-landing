package cache

import (
	"fmt"
	"sort"
	"sync"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
)

// ServiceInfo represents a service that appears on the landing page
type ServiceInfo struct {
	UID               string             `json:"uid"`
	Name              string             `json:"name"`
	Namespace         string             `json:"namespace"`
	DisplayName       string             `json:"displayName"`
	Description       string             `json:"description"`
	URL               string             `json:"url"`
	Icon              string             `json:"icon"`
	Category          string             `json:"category"`
	Priority          int                `json:"priority"`
	Visibility        string             `json:"visibility"`
	RequiredGroups    []string           `json:"requiredGroups,omitempty"`
	Health            *HealthStatus      `json:"health,omitempty"`
	HealthCheckConfig *HealthCheckConfig `json:"-"` // not serialised; used by the health checker
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

	service := &ServiceInfo{
		UID:               a.UID,
		Name:              a.Name,
		Namespace:         a.Namespace,
		DisplayName:       lp.DisplayName,
		Description:       lp.Description,
		URL:               buildURL(a),
		Icon:              lp.Icon,
		Category:          lp.Category,
		Priority:          priority,
		Visibility:        visibility,
		RequiredGroups:    lp.RequiredGroups,
		Health:            c.preserveHealthStatus(a.UID),
		HealthCheckConfig: buildHealthCheckConfig(a),
	}

	c.mu.Lock()
	defer c.mu.Unlock()
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

func (c *ServiceCache) preserveHealthStatus(uid string) *HealthStatus {
	if existing := c.services[uid]; existing != nil && existing.Health != nil {
		return existing.Health
	}
	return &HealthStatus{
		Status: "unknown",
	}
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
	path := hc.Path
	if path == "" {
		path = "/"
	}
	interval := hc.IntervalSeconds
	if interval <= 0 {
		interval = 30
	}
	timeout := hc.TimeoutSeconds
	if timeout <= 0 {
		timeout = 5
	}
	// Probe the Kubernetes service directly using in-cluster DNS so the health
	// check bypasses the ingress/gateway and always uses HTTP regardless of
	// whether TLS is configured for external access.
	serviceName := a.ServiceName
	if serviceName == "" {
		serviceName = a.Name
	}
	servicePort := a.ServicePort
	if servicePort == 0 {
		servicePort = 80
	}
	return &HealthCheckConfig{
		ProbeURL:        fmt.Sprintf("http://%s.%s:%d%s", serviceName, a.Namespace, servicePort, path),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
	}
}
