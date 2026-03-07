// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package app defines the internal domain model for Nebari application
// service-discovery. These types decouple the ServiceCache and downstream
// consumers from the Kubernetes API types (NebariApp CRD), so the cache
// layer never imports k8s machinery directly.
package app

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

	// Icon identifies the service icon (built-in name or image URL).
	Icon string

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
}
