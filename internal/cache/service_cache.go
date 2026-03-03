package cache

import (
	"sort"
	"sync"
	"time"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
)

// ServiceInfo represents a service that appears on the landing page
type ServiceInfo struct {
	UID            string        `json:"uid"`
	Name           string        `json:"name"`
	Namespace      string        `json:"namespace"`
	DisplayName    string        `json:"displayName"`
	Description    string        `json:"description"`
	URL            string        `json:"url"`
	Icon           string        `json:"icon"`
	Category       string        `json:"category"`
	Priority       int           `json:"priority"`
	Visibility     string        `json:"visibility"`
	RequiredGroups []string      `json:"requiredGroups,omitempty"`
	Health         *HealthStatus `json:"health,omitempty"`
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
	visibility := "authenticated"
	if lp.Visibility != "" {
		visibility = lp.Visibility
	}

	service := &ServiceInfo{
		UID:            a.UID,
		Name:           a.Name,
		Namespace:      a.Namespace,
		DisplayName:    lp.DisplayName,
		Description:    lp.Description,
		URL:            buildURL(a),
		Icon:           lp.Icon,
		Category:       lp.Category,
		Priority:       priority,
		Visibility:     visibility,
		RequiredGroups: lp.RequiredGroups,
		Health:         c.preserveHealthStatus(a.UID),
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
