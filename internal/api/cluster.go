package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/nebari-dev/nebari-landing/internal/health"
)

// HealthEventSource provides a recent history of service health transitions.
// *health.HealthChecker satisfies this interface.
type HealthEventSource interface {
	RecentEvents(n int) []health.HealthEvent
}

// WithHealthChecker attaches a health event source to the handler, enabling
// the GET /api/v1/cluster/services endpoint to include recent activity.
func WithHealthChecker(hc HealthEventSource) HandlerOption {
	return func(h *Handler) { h.healthChecker = hc }
}

// clusterServicesResponse is the JSON body returned by GET /api/v1/cluster/services.
type clusterServicesResponse struct {
	Services struct {
		Total     int `json:"total"`
		Healthy   int `json:"healthy"`
		Unhealthy int `json:"unhealthy"`
		Unknown   int `json:"unknown"`
	} `json:"services"`
	RecentActivity []clusterActivityEntry `json:"recent_activity"`
}

type clusterActivityEntry struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// handleClusterServices handles GET /api/v1/cluster/services.
//
// Returns a service health summary derived from the in-memory ServiceCache
// plus a chronological (newest-first) list of recent health transitions
// recorded by the HealthChecker since the webapi started.
//
// No auth is required — the data does not contain URLs or user-specific
// information, only aggregate counts and service names.
func (h *Handler) handleClusterServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services := h.cache.GetAll()

	var resp clusterServicesResponse
	resp.RecentActivity = []clusterActivityEntry{} // never null in JSON

	for _, svc := range services {
		resp.Services.Total++
		switch {
		case svc.Health == nil || svc.Health.Status == "" || svc.Health.Status == "unknown":
			resp.Services.Unknown++
		case svc.Health.Status == "healthy":
			resp.Services.Healthy++
		default:
			resp.Services.Unhealthy++
		}
	}

	if h.healthChecker != nil {
		for _, ev := range h.healthChecker.RecentEvents(20) {
			resp.RecentActivity = append(resp.RecentActivity, clusterActivityEntry{
				Name:      ev.Name,
				Status:    ev.Status,
				Timestamp: ev.Timestamp,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error(err, "Failed to encode cluster services response")
	}
}
