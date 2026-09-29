// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package packs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	VersionCurrent = "current"
	VersionBehind  = "behind"
	VersionAhead   = "ahead"
	VersionUnknown = "unknown"
)

// helmIndex is the subset of a Helm repository index.yaml we keep: chart
// name → published versions. It is filled by a streaming line scanner rather
// than a YAML parser, because a large repository index (tens of thousands of
// entries) as a YAML node tree costs far more memory than the webapi has.
type helmIndex struct {
	Entries map[string][]string
}

// maxIndexBytes bounds how much of an index we are willing to stream.
const maxIndexBytes = 32 << 20

// parseIndex scans a Helm index.yaml. `helm repo index` always emits:
//
//	entries:
//	  <chart>:
//	  - ...
//	    version: 1.2.3
//
// so a chart key is a line indented by exactly two spaces ending in ":" under
// "entries:", and each version is a "version:" scalar at deeper indentation.
func parseIndex(r io.Reader) (*helmIndex, error) {
	idx := &helmIndex{Entries: map[string][]string{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	inEntries := false
	chart := ""
	for sc.Scan() {
		line := sc.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		if line[0] != ' ' {
			inEntries = strings.HasPrefix(line, "entries:")
			chart = ""
			continue
		}
		if !inEntries {
			continue
		}
		if strings.HasPrefix(line, "  ") && line[2] != ' ' && line[2] != '-' && strings.HasSuffix(strings.TrimRight(line, " "), ":") {
			chart = strings.TrimSuffix(strings.TrimSpace(line), ":")
			chart = strings.Trim(chart, "\"'")
			continue
		}
		if chart == "" {
			continue
		}
		t := strings.TrimLeft(line, " -")
		if strings.HasPrefix(t, "version:") {
			v := strings.Trim(strings.TrimSpace(strings.TrimPrefix(t, "version:")), "\"'")
			if v != "" {
				idx.Entries[chart] = append(idx.Entries[chart], v)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return idx, nil
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

	mu     sync.Mutex
	cache  map[string]*indexEntry
	flight map[string]*sync.Mutex
}

// NewVersionSource returns a source with a short request timeout and a
// ten-minute cache per repository.
func NewVersionSource() *VersionSource {
	return &VersionSource{
		client: &http.Client{Timeout: 5 * time.Second},
		ttl:    10 * time.Minute,
		errTTL: 2 * time.Minute,
		cache:  map[string]*indexEntry{},
		flight: map[string]*sync.Mutex{},
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
		sv, err := semver.NewVersion(strings.TrimPrefix(e, "v"))
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

func (v *VersionSource) cached(key string) (*helmIndex, error, bool) {
	e, ok := v.cache[key]
	if !ok {
		return nil, nil, false
	}
	ttl := v.ttl
	if e.err != nil {
		ttl = v.errTTL
	}
	if time.Since(e.fetchedAt) < ttl {
		return e.index, e.err, true
	}
	return nil, nil, false
}

func (v *VersionSource) index(ctx context.Context, repoURL string) (*helmIndex, error) {
	key := strings.TrimRight(repoURL, "/")
	v.mu.Lock()
	if idx, err, ok := v.cached(key); ok {
		v.mu.Unlock()
		return idx, err
	}
	fl, ok := v.flight[key]
	if !ok {
		fl = &sync.Mutex{}
		v.flight[key] = fl
	}
	v.mu.Unlock()

	// One fetch per repository at a time; late arrivals get the fresh cache.
	fl.Lock()
	defer fl.Unlock()
	v.mu.Lock()
	if idx, err, ok := v.cached(key); ok {
		v.mu.Unlock()
		return idx, err
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
	idx, err := parseIndex(io.LimitReader(resp.Body, maxIndexBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", url, err)
	}
	return idx, nil
}
