// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"net/http"

	"github.com/nebari-dev/nebari-landing/internal/packs"
)

// PackLister reads ArgoCD Applications for the packs view.
type PackLister interface {
	ListArgoApps(ctx context.Context) ([]packs.ArgoApp, error)
}

// WithPackLister attaches the ArgoCD reader behind /api/v1/admin/packs.
func WithPackLister(l PackLister) HandlerOption {
	return func(h *Handler) { h.packLister = l }
}

// WithPackVersionSource enables latest-version lookups against each pack's
// Helm repository index (cached; unknown for git and OCI sources).
func WithPackVersionSource(v *packs.VersionSource) HandlerOption {
	return func(h *Handler) { h.packVersions = v }
}

// AdminPacksResponse is the body of GET /api/v1/admin/packs.
type AdminPacksResponse struct {
	// ArgoCDAvailable is false when Applications could not be listed; packs
	// are then derived from NebariApp labels only and carry no `argo` block.
	ArgoCDAvailable bool         `json:"argocdAvailable"`
	Error           string       `json:"error,omitempty"`
	Packs           []packs.Pack `json:"packs"`
}

func (h *Handler) buildPacks(ctx context.Context) AdminPacksResponse {
	out := AdminPacksResponse{Packs: []packs.Pack{}}
	var apps []packs.ArgoApp
	if h.packLister != nil {
		if list, err := h.packLister.ListArgoApps(ctx); err == nil {
			apps = list
			out.ArgoCDAvailable = true
		} else {
			out.Error = err.Error()
			log.Info("Packs: ArgoCD applications unavailable, using NebariApp labels", "error", err.Error())
		}
	}
	out.Packs = packs.Build(apps, h.cache.GetAll())
	packs.Annotate(ctx, h.packVersions, out.Packs)
	return out
}

// handleAdminListPacks serves GET /api/v1/admin/packs.
//
//	@Summary		List software packs (admin)
//	@Description	One row per ArgoCD Application labeled part-of=nebari-packs (tier "pack") or nebari-foundational (tier "platform"), joined with the landing-page services it owns via the ArgoCD tracking-id annotation or destination namespace. Degrades to NebariApp Helm labels when ArgoCD is not readable.
//	@Tags			admin
//	@Produce		json
//	@Success		200	{object}	AdminPacksResponse
//	@Failure		403	{string}	string	"Forbidden: admin group required"
//	@Security		BearerAuth
//	@Router			/admin/packs [get]
func (h *Handler) handleAdminListPacks(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.buildPacks(r.Context()))
}

// handleAdminGetPack serves GET /api/v1/admin/packs/{name}.
//
//	@Summary		Get a software pack (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			name	path		string	true	"Pack (ArgoCD Application) name"
//	@Success		200		{object}	packs.Pack
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/packs/{name} [get]
func (h *Handler) handleAdminGetPack(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	name := r.PathValue("name")
	for _, p := range h.buildPacks(r.Context()).Packs {
		if p.Name == name {
			writeJSON(w, http.StatusOK, p)
			return
		}
	}
	writeAdminError(w, http.StatusNotFound, "pack not found")
}
