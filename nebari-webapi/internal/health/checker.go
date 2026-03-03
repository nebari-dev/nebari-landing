package health

import (
	"context"
	"time"

	"github.com/nebari-dev/nebari-landing/nebari-webapi/internal/cache"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("health-checker")

// HealthChecker performs periodic health checks on services
type HealthChecker struct {
	cache    *cache.ServiceCache
	interval time.Duration
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(serviceCache *cache.ServiceCache, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		cache:    serviceCache,
		interval: interval,
	}
}

// Start starts the health checker
func (h *HealthChecker) Start(ctx context.Context) {
	log.Info("Health checker started", "interval", h.interval)
	// TODO: Implement actual health checking logic
	<-ctx.Done()
	log.Info("Health checker stopped")
}
