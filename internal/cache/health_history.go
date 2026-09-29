// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"sort"
	"sync"
	"time"
)

// HealthSample is one probe outcome kept in the rolling history.
type HealthSample struct {
	At        time.Time `json:"at"`
	Status    string    `json:"status"`
	LatencyMS *int      `json:"latencyMs,omitempty"`
}

// HealthBucket aggregates the samples that fall into one time slot.
type HealthBucket struct {
	Start     time.Time `json:"start"`
	Total     int       `json:"total"`
	Healthy   int       `json:"healthy"`
	Unhealthy int       `json:"unhealthy"`
	Unknown   int       `json:"unknown"`
	// P50LatencyMS is the median probe latency of the bucket, nil when no
	// sample in it carried a latency.
	P50LatencyMS *int `json:"p50LatencyMs"`
}

// BucketHealth slots samples into n equal buckets spanning [from, to). Samples
// outside the window are ignored. Buckets with no samples have Total 0.
func BucketHealth(samples []HealthSample, from, to time.Time, n int) []HealthBucket {
	if n <= 0 || !to.After(from) {
		return nil
	}
	width := to.Sub(from) / time.Duration(n)
	out := make([]HealthBucket, n)
	lat := make([][]int, n)
	for i := range out {
		out[i].Start = from.Add(time.Duration(i) * width)
	}
	for _, s := range samples {
		if s.At.Before(from) || !s.At.Before(to) {
			continue
		}
		i := int(s.At.Sub(from) / width)
		if i >= n {
			i = n - 1
		}
		b := &out[i]
		b.Total++
		switch s.Status {
		case "healthy":
			b.Healthy++
		case "unhealthy":
			b.Unhealthy++
		default:
			b.Unknown++
		}
		if s.LatencyMS != nil {
			lat[i] = append(lat[i], *s.LatencyMS)
		}
	}
	for i := range out {
		if len(lat[i]) == 0 {
			continue
		}
		sort.Ints(lat[i])
		v := lat[i][len(lat[i])/2]
		out[i].P50LatencyMS = &v
	}
	return out
}

// HealthHistoryLimit is how many samples are retained per service. At the
// default 30s probe interval this covers roughly 24 hours.
const HealthHistoryLimit = 2880

// healthHistory is a per-service ring of the most recent probe outcomes. It
// lives beside the cache rather than in ServiceInfo so the service JSON
// served to the landing page stays small.
type healthHistory struct {
	mu      sync.RWMutex
	samples map[string][]HealthSample
}

func newHealthHistory() *healthHistory {
	return &healthHistory{samples: make(map[string][]HealthSample)}
}

func (h *healthHistory) record(uid string, s HealthSample) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := append(h.samples[uid], s)
	if len(list) > HealthHistoryLimit {
		list = list[len(list)-HealthHistoryLimit:]
	}
	h.samples[uid] = list
}

func (h *healthHistory) get(uid string) []HealthSample {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := h.samples[uid]
	out := make([]HealthSample, len(list))
	copy(out, list)
	return out
}

func (h *healthHistory) drop(uid string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.samples, uid)
}

// HealthHistory returns the retained probe samples for a service, oldest
// first. Empty when the service is unknown or has not been probed yet.
func (c *ServiceCache) HealthHistory(uid string) []HealthSample {
	return c.history.get(uid)
}

// HealthSummary aggregates a sample window into the figures the admin UI
// shows next to a service.
type HealthSummary struct {
	// Samples is how many probes the window holds.
	Samples int `json:"samples"`
	// UptimePercent is the share of samples that were healthy, or nil when
	// there are no samples.
	UptimePercent *float64 `json:"uptimePercent"`
	// StreakStatus / StreakSince describe the run of identical outcomes that
	// ends with the latest sample.
	StreakStatus string     `json:"streakStatus,omitempty"`
	StreakSince  *time.Time `json:"streakSince,omitempty"`
	// WindowStart is the oldest sample's time, when any exist.
	WindowStart *time.Time `json:"windowStart,omitempty"`
}

// SummarizeHealth computes uptime and the current streak for a window.
func SummarizeHealth(samples []HealthSample) HealthSummary {
	out := HealthSummary{Samples: len(samples)}
	if len(samples) == 0 {
		return out
	}
	healthy := 0
	for _, s := range samples {
		if s.Status == "healthy" {
			healthy++
		}
	}
	pct := float64(healthy) * 100 / float64(len(samples))
	out.UptimePercent = &pct
	start := samples[0].At
	out.WindowStart = &start

	last := samples[len(samples)-1]
	out.StreakStatus = last.Status
	since := last.At
	for i := len(samples) - 1; i >= 0 && samples[i].Status == last.Status; i-- {
		since = samples[i].At
	}
	out.StreakSince = &since
	return out
}
