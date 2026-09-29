// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"net/http"
	"time"

	"github.com/nebari-dev/nebari-landing/internal/accessrequests"
	"github.com/nebari-dev/nebari-landing/internal/keycloak"
)

// OverviewUsers summarises the realm's accounts.
type OverviewUsers struct {
	Total           int `json:"total"`
	Enabled         int `json:"enabled"`
	Disabled        int `json:"disabled"`
	WithoutGroups   int `json:"withoutGroups"`
	CreatedLast7Day int `json:"createdLast7Days"`
}

// OverviewServices summarises the landing-page services and their gates.
type OverviewServices struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
	Unknown   int `json:"unknown"`
	Public    int `json:"public"`
	Gated     int `json:"gated"`
}

// OverviewAccessRequests counts access requests by status.
type OverviewAccessRequests struct {
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Denied   int `json:"denied"`
}

// AdminOverview is the body of GET /api/v1/admin/overview.
type AdminOverview struct {
	GeneratedAt time.Time `json:"generatedAt"`
	// IdentityAvailable is false when no Keycloak admin client is configured;
	// user/group/role/session figures are then zero.
	IdentityAvailable bool             `json:"identityAvailable"`
	Users             OverviewUsers    `json:"users"`
	Groups            int              `json:"groups"`
	Roles             int              `json:"roles"`
	Services          OverviewServices `json:"services"`
	// AccessRequestsAvailable is false when the access-request store is off.
	AccessRequestsAvailable bool                   `json:"accessRequestsAvailable"`
	AccessRequests          OverviewAccessRequests `json:"accessRequests"`
	// Sessions is who is online right now (distinct users, raw sessions and
	// the per-client breakdown), or null when Keycloak could not be read.
	Sessions *keycloak.SessionStats `json:"sessions"`
}

// handleAdminOverview serves GET /api/v1/admin/overview.
//
//	@Summary		Admin overview figures
//	@Description	Headline counts for the admin dashboard: accounts, groups, roles, service health and gates, access requests by status, and live Keycloak sessions. Figures that depend on an unavailable backend are zero and flagged via the *Available booleans. Admin-only.
//	@Tags			admin
//	@Produce		json
//	@Success		200	{object}	AdminOverview
//	@Failure		403	{string}	string	"Forbidden: admin group required"
//	@Security		BearerAuth
//	@Router			/admin/overview [get]
func (h *Handler) handleAdminOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	out := AdminOverview{GeneratedAt: time.Now().UTC()}

	for _, s := range h.cache.GetAll() {
		out.Services.Total++
		switch {
		case s.Health == nil || s.Health.Status == "" || s.Health.Status == "unknown":
			out.Services.Unknown++
		case s.Health.Status == "healthy":
			out.Services.Healthy++
		default:
			out.Services.Unhealthy++
		}
		if s.Visibility == "public" {
			out.Services.Public++
		} else if len(s.RequiredGroups) > 0 {
			out.Services.Gated++
		}
	}

	if h.identity != nil {
		out.IdentityAvailable = true
		weekAgo := time.Now().Add(-7 * 24 * time.Hour)
		if users, err := h.identity.ListUsers(ctx); err == nil {
			for _, u := range users {
				out.Users.Total++
				if u.Enabled {
					out.Users.Enabled++
				} else {
					out.Users.Disabled++
				}
				if len(u.Groups) == 0 {
					out.Users.WithoutGroups++
				}
				if t, perr := time.Parse(time.RFC3339, u.CreatedAt); perr == nil && t.After(weekAgo) {
					out.Users.CreatedLast7Day++
				}
			}
		} else {
			log.Error(err, "Overview: listing users failed")
			out.IdentityAvailable = false
		}
		if groups, err := h.identity.ListGroups(ctx); err == nil {
			out.Groups = len(groups)
		}
		if roles, err := h.identity.ListRoles(ctx); err == nil {
			for _, role := range roles {
				if !role.BuiltIn {
					out.Roles++
				}
			}
		}
		if stats, err := h.identity.ActiveSessions(ctx); err == nil {
			out.Sessions = stats
		} else {
			log.Info("Overview: active session count unavailable", "error", err.Error())
		}
	}

	if h.accessRequestStore != nil {
		out.AccessRequestsAvailable = true
		if reqs, err := h.accessRequestStore.ListAll(); err == nil {
			for _, req := range reqs {
				switch req.Status {
				case accessrequests.StatusPending:
					out.AccessRequests.Pending++
				case accessrequests.StatusApproved:
					out.AccessRequests.Approved++
				case accessrequests.StatusDenied:
					out.AccessRequests.Denied++
				}
			}
		} else {
			log.Error(err, "Overview: listing access requests failed")
		}
	}

	writeJSON(w, http.StatusOK, out)
}
