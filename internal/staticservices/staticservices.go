// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package staticservices parses landing-page entries that are configured on
// the webapi directly instead of being discovered from NebariApp CRs.
//
// They exist for services that cannot (or should not) carry a NebariApp, the
// motivating case being Keycloak: it is the identity provider, so it cannot
// have spec.auth enabled, and the NebariApp-derived visibility would always be
// "public". A static entry decouples card visibility (discovery) from gateway
// enforcement: the card is gated by visibility/requiredGroups through the same
// access policy as every other service, while the target keeps enforcing its
// own authentication.
package staticservices

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"

	sdapp "github.com/nebari-dev/nebari-landing/internal/app"
)

// UIDPrefix is prepended to every static entry's ID to form its cache UID.
// Kubernetes UIDs are RFC 4122 UUIDs, so prefixed IDs cannot collide with a
// NebariApp, and the watcher never evicts them.
const UIDPrefix = "static-"

var idPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Service is one static landing-page entry, as rendered by the Helm chart.
type Service struct {
	ID             string       `json:"id"`
	DisplayName    string       `json:"displayName,omitempty"`
	Description    string       `json:"description,omitempty"`
	URL            string       `json:"url"`
	Icon           string       `json:"icon,omitempty"`
	IconLight      string       `json:"iconLight,omitempty"`
	IconDark       string       `json:"iconDark,omitempty"`
	Category       string       `json:"category,omitempty"`
	Priority       int          `json:"priority,omitempty"`
	Visibility     string       `json:"visibility,omitempty"`
	RequiredGroups []string     `json:"requiredGroups,omitempty"`
	HealthCheck    *HealthCheck `json:"healthCheck,omitempty"`
}

// HealthCheck configures an optional probe for a static entry. Unlike
// NebariApp health checks there is no Service reference to build the address
// from, so the probe URL is given in full.
type HealthCheck struct {
	URL             string `json:"url"`
	IntervalSeconds int    `json:"intervalSeconds,omitempty"`
	TimeoutSeconds  int    `json:"timeoutSeconds,omitempty"`
}

// Parse decodes and validates a JSON array of static services. Empty input
// yields no services. Unknown fields are rejected so a typo in the chart
// values (e.g. "requiredGroup") fails loudly instead of silently dropping a
// visibility gate.
func Parse(data []byte) ([]Service, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var services []Service
	if err := dec.Decode(&services); err != nil {
		return nil, fmt.Errorf("decoding static services: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("decoding static services: unexpected data after the JSON array")
	}

	seen := make(map[string]bool, len(services))
	for i := range services {
		s := &services[i]
		if err := s.validate(); err != nil {
			return nil, fmt.Errorf("static service %d (%q): %w", i, s.ID, err)
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("static service %q: duplicate id", s.ID)
		}
		seen[s.ID] = true
	}
	return services, nil
}

func (s *Service) validate() error {
	if !idPattern.MatchString(s.ID) {
		return fmt.Errorf("id must be a lowercase DNS label (got %q)", s.ID)
	}
	if err := validateHTTPURL(s.URL); err != nil {
		return fmt.Errorf("url: %w", err)
	}
	switch s.Visibility {
	case "":
		// Default to private: a static entry that forgets its visibility
		// should not be shown to anonymous visitors.
		s.Visibility = "private"
	case "private":
	case "public":
		if len(s.RequiredGroups) > 0 {
			return fmt.Errorf("requiredGroups has no effect when visibility is public")
		}
	default:
		return fmt.Errorf("visibility must be public or private (got %q)", s.Visibility)
	}
	if s.HealthCheck != nil {
		if err := validateHTTPURL(s.HealthCheck.URL); err != nil {
			return fmt.Errorf("healthCheck.url: %w", err)
		}
	}
	return nil
}

func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("must be an absolute http(s) URL (got %q)", raw)
	}
	return nil
}

// App converts the entry into the internal domain model consumed by the
// service cache.
func (s Service) App() *sdapp.App {
	displayName := s.DisplayName
	if displayName == "" {
		displayName = s.ID
	}

	var hc *sdapp.HealthCheck
	if s.HealthCheck != nil {
		hc = &sdapp.HealthCheck{
			Enabled:         true,
			URL:             s.HealthCheck.URL,
			IntervalSeconds: s.HealthCheck.IntervalSeconds,
			TimeoutSeconds:  s.HealthCheck.TimeoutSeconds,
		}
	}

	return &sdapp.App{
		UID:    UIDPrefix + s.ID,
		Name:   s.ID,
		Static: true,
		LandingPage: &sdapp.LandingPage{
			Enabled:        true,
			DisplayName:    displayName,
			Description:    s.Description,
			Icon:           s.Icon,
			IconLight:      s.IconLight,
			IconDark:       s.IconDark,
			Category:       s.Category,
			Priority:       s.Priority,
			Visibility:     s.Visibility,
			RequiredGroups: s.RequiredGroups,
			ExternalURL:    s.URL,
			HealthCheck:    hc,
		},
	}
}
