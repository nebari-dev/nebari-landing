// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package packs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"
)

const (
	VersionCurrent = "current"
	VersionBehind  = "behind"
	VersionAhead   = "ahead"
	VersionUnknown = "unknown"
)

// helmIndex is the subset of a Helm repository index.yaml we read.
type helmIndex struct {
	Entries map[string][]struct {
		Version    string `json:"version" yaml:"version"`
		AppVersion string `json:"appVersion" yaml:"appVersion"`
	} `yaml:"entries"`
}

type indexEntry struct {
	fetchedAt time.Time
	index     *helmIndex
	err       error
}

// VersionSource resolves the latest published version of a chart by reading
// the Helm repository index the pack is installed from. Indexes are cached
// per repository; OCI and git sources are reported as unknown.
type VersionSource struct {
	client *http.Client
	ttl    time.Duration
	errTTL time.Duration

	mu    sync.Mutex
	cache map[string]*indexEntry
}

// NewVersionSource returns a source with a short request timeout and a
// ten-minute cache per repository.
func NewVersionSource() *VersionSource {
	return &VersionSource{
		client: &http.Client{Timeout: 5 * time.Second},
		ttl:    10 * time.Minute,
		errTTL: 2 * time.Minute,
		cache:  map[string]*indexEntry{},
	}
}

// Latest returns the newest published version of chart in the given Helm
// repository, and whether the lookup was possible. Pre-release versions are
// only considered when the installed version is itself a pre-release.
func (v *VersionSource) Latest(ctx context.Context, repoURL, chart, installed string) (string, bool) {
	if v == nil || chart == "" || (!strings.HasPrefix(repoURL, "https://") && !strings.HasPrefix(repoURL, "http://")) {
		return "", false
	}
	idx, err := v.index(ctx, repoURL)
	if err != nil || idx == nil {
		return "", false
	}
	entries, ok := idx.Entries[chart]
	if !ok || len(entries) == 0 {
		return "", false
	}
	cur, curErr := semver.NewVersion(strings.TrimPrefix(installed, "v"))
	allowPre := curErr == nil && cur.Prerelease() != ""
	var best *semver.Version
	for _, e := range entries {
		sv, err := semver.NewVersion(strings.TrimPrefix(e.Version, "v"))
		if err != nil {
			continue
		}
		if sv.Prerelease() != "" && !allowPre {
			continue
		}
		if best == nil || sv.GreaterThan(best) {
			best = sv
		}
	}
	if best == nil {
		return "", false
	}
	return best.Original(), true
}

// CompareVersions classifies installed against latest.
func CompareVersions(installed, latest string) string {
	if installed == "" || latest == "" {
		return VersionUnknown
	}
	cur, err1 := semver.NewVersion(strings.TrimPrefix(installed, "v"))
	lat, err2 := semver.NewVersion(strings.TrimPrefix(latest, "v"))
	if err1 != nil || err2 != nil {
		return VersionUnknown
	}
	switch {
	case cur.LessThan(lat):
		return VersionBehind
	case cur.GreaterThan(lat):
		return VersionAhead
	}
	return VersionCurrent
}

func (v *VersionSource) index(ctx context.Context, repoURL string) (*helmIndex, error) {
	key := strings.TrimRight(repoURL, "/")
	v.mu.Lock()
	if e, ok := v.cache[key]; ok {
		ttl := v.ttl
		if e.err != nil {
			ttl = v.errTTL
		}
		if time.Since(e.fetchedAt) < ttl {
			v.mu.Unlock()
			return e.index, e.err
		}
	}
	v.mu.Unlock()

	idx, err := v.fetch(ctx, key+"/index.yaml")
	v.mu.Lock()
	v.cache[key] = &indexEntry{fetchedAt: time.Now(), index: idx, err: err}
	v.mu.Unlock()
	return idx, err
}

func (v *VersionSource) fetch(ctx context.Context, url string) (*helmIndex, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var idx helmIndex
	if err := yaml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", url, err)
	}
	return &idx, nil
}
