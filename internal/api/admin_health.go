// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/nebari-dev/nebari-landing/internal/cache"
)

// ServiceHealthSeries is one service's bucketed probe history.
type ServiceHealthSeries struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	// LatencyMS is the latest probe round-trip.
	LatencyMS *int `json:"latencyMs"`
	// UptimePercent is over the requested window.
	UptimePercent *float64 `json:"uptimePercent"`
	// P50LatencyMS / P95LatencyMS are over the requested window.
	P50LatencyMS *int                 `json:"p50LatencyMs"`
	P95LatencyMS *int                 `json:"p95LatencyMs"`
	Incidents    []cache.Incident     `json:"incidents"`
	Buckets      []cache.HealthBucket `json:"buckets"`
}

// AdminHealthSeriesResponse is the body of GET /api/v1/admin/services/health.
type AdminHealthSeriesResponse struct {
	From     time.Time             `json:"from"`
	To       time.Time             `json:"to"`
	Buckets  int                   `json:"buckets"`
	Services []ServiceHealthSeries `json:"services"`
}

// handleAdminHealthSeries serves GET /api/v1/admin/services/health.
//
//	@Summary		Bucketed health and latency for every probed service (admin)
//	@Description	Slots each service's retained probe samples into equal time buckets over a shared window (default 24h, 48 buckets) with per-bucket status counts and median latency. Services without a health check are omitted.
//	@Tags			admin
//	@Produce		json
//	@Param			hours	query		int	false	"Window length in hours (1–168, default 24)"
//	@Param			buckets	query		int	false	"Number of buckets (4–288, default 48)"
//	@Success		200		{object}	AdminHealthSeriesResponse
//	@Failure		403		{string}	string	"Forbidden: admin group required"
//	@Security		BearerAuth
//	@Router			/admin/services/health [get]
func (h *Handler) handleAdminHealthSeries(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	hours := queryInt(r, "hours", 24, 1, 168)
	n := queryInt(r, "buckets", 48, 4, 288)
	to := time.Now().UTC()
	from := to.Add(-time.Duration(hours) * time.Hour)

	out := AdminHealthSeriesResponse{From: from, To: to, Buckets: n, Services: []ServiceHealthSeries{}}
	for _, s := range h.cache.GetAll() {
		samples := h.cache.HealthHistory(s.UID)
		// Only services with a probe configured (or a retained history from
		// one) belong on the chart.
		if s.HealthCheckConfig == nil && len(samples) == 0 {
			continue
		}
		series := ServiceHealthSeries{
			ID: s.UID, DisplayName: s.DisplayName, Status: "unknown",
			Buckets: cache.BucketHealth(samples, from, to, n),
		}
		if s.Health != nil {
			series.Status = s.Health.Status
			series.LatencyMS = s.Health.LatencyMS
		}
		inWindow := samples[:0:0]
		for _, smp := range samples {
			if !smp.At.Before(from) && smp.At.Before(to) {
				inWindow = append(inWindow, smp)
			}
		}
		series.UptimePercent = cache.SummarizeHealth(inWindow).UptimePercent
		series.P50LatencyMS, series.P95LatencyMS = cache.LatencyPercentiles(samples, from, to)
		series.Incidents = cache.Incidents(samples, from, to)
		out.Services = append(out.Services, series)
	}
	writeJSON(w, http.StatusOK, out)
}

func queryInt(r *http.Request, key string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
