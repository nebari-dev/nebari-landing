// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package app defines the internal domain model for Nebari application
// service-discovery. These types decouple the ServiceCache and downstream
// consumers from the Kubernetes API types (NebariApp CRD), so the cache
// layer never imports k8s machinery directly.
package app

import "encoding/json"

// ThemeIcon holds the icon URL(s) for a service. Use Plain for a single URL that
// works in both modes, or set Light/Dark for theme-aware variants.
// When both Plain and Light/Dark are set, Light/Dark take precedence.
type ThemeIcon struct {
	Plain string
	Light string
	Dark  string
}

// AsIconValue converts ThemeIcon to its JSON-serializable form. The two types
// are structurally identical; this cast makes the conversion explicit.
func (t ThemeIcon) AsIconValue() IconValue { return IconValue(t) }

// IconValue is the JSON-serializable icon for a service. It marshals to either a
// plain string or a {"light","dark"} object depending on which fields are set.
type IconValue struct {
	Plain string
	Light string
	Dark  string
}

func (v IconValue) MarshalJSON() ([]byte, error) {
	if v.Light != "" || v.Dark != "" {
		return json.Marshal(struct {
			Light string `json:"light,omitempty"`
			Dark  string `json:"dark,omitempty"`
		}{Light: v.Light, Dark: v.Dark})
	}
	return json.Marshal(v.Plain)
}

func (v *IconValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		v.Plain = s
		return nil
	}
	var obj struct {
		Light string `json:"light"`
		Dark  string `json:"dark"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	v.Light = obj.Light
	v.Dark = obj.Dark
	return nil
}

// String returns a single URL for contexts that require a plain string (e.g. notifications).
func (v IconValue) String() string {
	if v.Light != "" {
		return v.Light
	}
	if v.Dark != "" {
		return v.Dark
	}
	return v.Plain
}

// App is the internal representation of a Nebari application that participates
// in service discovery. It is derived from a NebariApp CR by the watcher and
// passed to the ServiceCache.
type App struct {
	// UID is the Kubernetes UID of the underlying NebariApp.
	UID string

	// Name is the name of the NebariApp CR.
	Name string

	// Namespace is the namespace of the NebariApp CR.
	Namespace string

	// Hostname is spec.hostname.
	Hostname string

	// TLSEnabled reflects whether TLS termination is configured
	// (spec.routing.tls.enabled != false).
	TLSEnabled bool

	// ServiceName is spec.service.name — the Kubernetes Service to probe for
	// health checks.
	ServiceName string

	// ServiceNamespace is spec.service.namespace — the namespace containing the
	// Kubernetes Service. Defaults to Namespace (the NebariApp's own namespace)
	// when not explicitly set, but must be specified when the target Service
	// lives in a different namespace (e.g. Keycloak in the keycloak namespace).
	ServiceNamespace string

	// ServicePort is spec.service.port — the port on the Kubernetes Service.
	ServicePort int

	// LandingPage holds the resolved landing-page configuration, or nil when
	// the application does not participate in service discovery.
	LandingPage *LandingPage
}

// LandingPage holds the resolved settings for an App that is listed on the
// Nebari landing page.
type LandingPage struct {
	// Enabled mirrors spec.landingPage.enabled.
	Enabled bool

	// DisplayName is the human-readable name shown on service cards.
	DisplayName string

	// Description is supplementary text for the service card.
	Description string

	// Icon holds the service icon. Set Plain for a single URL; set Light/Dark for theme-aware variants.
	Icon ThemeIcon

	// Category groups related services on the landing page.
	Category string

	// Priority controls sort order within a category (lower = higher priority).
	// Defaults to 100 when not explicitly set.
	Priority int

	// Visibility controls who can see this service.
	// Valid values:
	//   "public"  — visible to all users, no authentication required.
	//   "private" — visible only to authenticated users who are a member of at
	//               least one group listed in RequiredGroups. When RequiredGroups
	//               is empty, any authenticated user can see the service.
	// Default (when unset): "private".
	Visibility string

	// RequiredGroups lists Keycloak groups required when Visibility is "private".
	RequiredGroups []string

	// ExternalURL overrides the URL derived from Hostname.
	ExternalURL string

	// HealthCheck holds the health-probe configuration for this service.
	// When nil or Enabled is false, health status remains "unknown".
	HealthCheck *HealthCheck
}

// HealthCheck holds the health-probing configuration derived from
// spec.landingPage.healthCheck in the NebariApp CRD.
type HealthCheck struct {
	// Enabled mirrors spec.landingPage.healthCheck.enabled.
	Enabled bool

	// Path is the HTTP path to probe (e.g. "/healthz", "/").
	Path string

	// IntervalSeconds is how frequently to probe the service.
	// Defaults to 30 when not set.
	IntervalSeconds int

	// TimeoutSeconds is the per-probe HTTP timeout.
	// Defaults to 5 when not set.
	TimeoutSeconds int

	// Port overrides the service port for the health probe. Useful when the
	// target exposes health endpoints on a separate management port (e.g.
	// Keycloak X exposes /health/ready on port 9000, not the main 8080).
	// When 0, spec.service.port is used.
	Port int
}
