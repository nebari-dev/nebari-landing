// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/nebari-dev/nebari-landing/internal/cache"
	"github.com/nebari-dev/nebari-landing/internal/keycloak"
)

// IdentityAdmin is the slice of the Keycloak admin client the identity
// endpoints depend on. *keycloak.Client is the production implementation;
// tests substitute an in-memory fake.
type IdentityAdmin interface {
	ListUsers(ctx context.Context) ([]keycloak.IdentityUser, error)
	GetUser(ctx context.Context, id string) (*keycloak.IdentityUser, error)
	SetUserEnabled(ctx context.Context, id string, enabled bool) error
	DeleteUser(ctx context.Context, id string) error
	AddUserToGroupByID(ctx context.Context, userID, groupID string) error
	RemoveUserFromGroup(ctx context.Context, userID, groupID string) error
	AssignRealmRole(ctx context.Context, userID, roleName string) error
	UnassignRealmRole(ctx context.Context, userID, roleName string) error

	ListGroups(ctx context.Context) ([]keycloak.IdentityGroup, error)
	GetGroup(ctx context.Context, id string) (*keycloak.IdentityGroup, error)
	CreateGroup(ctx context.Context, name, description string) (*keycloak.IdentityGroup, error)
	UpdateGroupDescription(ctx context.Context, id, description string) error
	DeleteGroup(ctx context.Context, id string) error
	AddRealmRoleToGroup(ctx context.Context, groupID, roleName string) error
	RemoveRealmRoleFromGroup(ctx context.Context, groupID, roleName string) error

	ListRoles(ctx context.Context) ([]keycloak.IdentityRole, error)
	CreateRole(ctx context.Context, name, description string) (*keycloak.IdentityRole, error)
	UpdateRoleDescription(ctx context.Context, name, description string) error
	DeleteRole(ctx context.Context, name string) error
}

// WithIdentityAdmin attaches the identity backend for the /admin/{users,
// groups,roles} endpoints. WithKeycloakAdminClient sets it automatically;
// this option exists for tests.
func WithIdentityAdmin(ia IdentityAdmin) HandlerOption {
	return func(h *Handler) { h.identity = ia }
}

// AdminUsersResponse is the body of GET /api/v1/admin/users.
type AdminUsersResponse struct {
	Users []keycloak.IdentityUser `json:"users"`
	Total int                     `json:"total"`
}

// AdminService is the admin-area view of a landing-page service: the routing
// gate declared in the NebariApp CR. Read-only — GitOps owns the CR.
type AdminService struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Namespace      string   `json:"namespace"`
	Category       string   `json:"category"`
	URL            string   `json:"url"`
	Visibility     string   `json:"visibility"`
	RequiredGroups []string `json:"requiredGroups"`
}

// AdminUserPatch is the body of PATCH /api/v1/admin/users/{id}.
type AdminUserPatch struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// AdminGroupInput is the body of POST/PATCH /api/v1/admin/groups[/{id}].
type AdminGroupInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// AdminRoleInput is the body of POST/PATCH /api/v1/admin/roles[/{name}].
type AdminRoleInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// BulkUsersRequest is the body of POST /api/v1/admin/users/bulk.
type BulkUsersRequest struct {
	UserIDs []string `json:"userIds"`
	// Action is one of addToGroup, removeFromGroup, assignRole, unassignRole, enable, disable.
	Action  string `json:"action"`
	GroupID string `json:"groupId,omitempty"`
	Role    string `json:"role,omitempty"`
}

// BulkUsersResponse is the body of POST /api/v1/admin/users/bulk.
type BulkUsersResponse struct {
	Updated int `json:"updated"`
}

// AdminError is the JSON error body for the identity endpoints.
type AdminError struct {
	Error string `json:"error"`
	// Services lists the service ids that still reference a group when a
	// delete is refused with 409.
	Services []string `json:"services,omitempty"`
}

var identifierRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error(err, "Failed to encode admin response")
	}
}

func writeAdminError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, AdminError{Error: msg})
}

// writeKeycloakError maps a Keycloak admin API failure onto an HTTP status.
func writeKeycloakError(w http.ResponseWriter, err error, what string) {
	switch {
	case keycloak.IsNotFound(err):
		writeAdminError(w, http.StatusNotFound, what+" not found")
	case keycloak.IsConflict(err):
		writeAdminError(w, http.StatusConflict, what+" already exists")
	default:
		log.Error(err, "Keycloak admin operation failed", "what", what)
		writeAdminError(w, http.StatusBadGateway, "Keycloak request failed: "+err.Error())
	}
}

// requireIdentityAdmin gates every identity endpoint: admin group required,
// 501 when no Keycloak admin client is configured.
func (h *Handler) requireIdentityAdmin(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := h.requireAdmin(w, r); !ok {
		return false
	}
	if h.identity == nil {
		writeAdminError(w, http.StatusNotImplemented,
			"Keycloak admin client not configured (set webapi.keycloak.adminSecretName)")
		return false
	}
	return true
}

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// registerIdentityRoutes wires the admin identity endpoints. Go 1.22 method
// patterns keep the dispatch declarative.
func (h *Handler) registerIdentityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/users", h.handleAdminListUsers)
	mux.HandleFunc("POST /api/v1/admin/users/bulk", h.handleAdminBulkUsers)
	mux.HandleFunc("GET /api/v1/admin/users/{id}", h.handleAdminGetUser)
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.handleAdminPatchUser)
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", h.handleAdminDeleteUser)
	mux.HandleFunc("PUT /api/v1/admin/users/{id}/groups/{groupId}", h.handleAdminUserGroup)
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}/groups/{groupId}", h.handleAdminUserGroup)
	mux.HandleFunc("PUT /api/v1/admin/users/{id}/roles/{role}", h.handleAdminUserRole)
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}/roles/{role}", h.handleAdminUserRole)

	mux.HandleFunc("GET /api/v1/admin/groups", h.handleAdminListGroups)
	mux.HandleFunc("POST /api/v1/admin/groups", h.handleAdminCreateGroup)
	mux.HandleFunc("GET /api/v1/admin/groups/{id}", h.handleAdminGetGroup)
	mux.HandleFunc("PATCH /api/v1/admin/groups/{id}", h.handleAdminPatchGroup)
	mux.HandleFunc("DELETE /api/v1/admin/groups/{id}", h.handleAdminDeleteGroup)
	mux.HandleFunc("PUT /api/v1/admin/groups/{id}/members/{userId}", h.handleAdminGroupMember)
	mux.HandleFunc("DELETE /api/v1/admin/groups/{id}/members/{userId}", h.handleAdminGroupMember)
	mux.HandleFunc("PUT /api/v1/admin/groups/{id}/roles/{role}", h.handleAdminGroupRole)
	mux.HandleFunc("DELETE /api/v1/admin/groups/{id}/roles/{role}", h.handleAdminGroupRole)

	mux.HandleFunc("GET /api/v1/admin/roles", h.handleAdminListRoles)
	mux.HandleFunc("POST /api/v1/admin/roles", h.handleAdminCreateRole)
	mux.HandleFunc("PATCH /api/v1/admin/roles/{name}", h.handleAdminPatchRole)
	mux.HandleFunc("DELETE /api/v1/admin/roles/{name}", h.handleAdminDeleteRole)

	mux.HandleFunc("GET /api/v1/admin/services", h.handleAdminListServices)
	mux.HandleFunc("GET /api/v1/admin/services/{id}", h.handleAdminGetService)
}

// --- users -----------------------------------------------------------------

// handleAdminListUsers serves GET /api/v1/admin/users.
//
//	@Summary		List users (admin)
//	@Description	Returns every realm user with direct group ids and realm roles. Optional filters: q (name/username/email substring), group (group id), role (realm role), enabled (true|false). Admin-only.
//	@Tags			admin
//	@Produce		json
//	@Param			q		query		string	false	"Substring match on name, username or email"
//	@Param			group	query		string	false	"Only users in this group id"
//	@Param			role	query		string	false	"Only users with this direct realm role"
//	@Param			enabled	query		bool	false	"Filter by enabled flag"
//	@Success		200		{object}	AdminUsersResponse
//	@Failure		401		{string}	string	"Unauthorized"
//	@Failure		403		{string}	string	"Forbidden: admin group required"
//	@Failure		501		{object}	AdminError	"Keycloak admin client not configured"
//	@Failure		502		{object}	AdminError	"Keycloak request failed"
//	@Security		BearerAuth
//	@Router			/admin/users [get]
func (h *Handler) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	users, err := h.identity.ListUsers(r.Context())
	if err != nil {
		writeKeycloakError(w, err, "users")
		return
	}
	q := r.URL.Query()
	needle := strings.ToLower(q.Get("q"))
	group := q.Get("group")
	role := q.Get("role")
	enabled := q.Get("enabled")

	filtered := make([]keycloak.IdentityUser, 0, len(users))
	for _, u := range users {
		if needle != "" && !userMatches(u, needle) {
			continue
		}
		if group != "" && !contains(u.Groups, group) {
			continue
		}
		if role != "" && !contains(u.Roles, role) {
			continue
		}
		if (enabled == "true" && !u.Enabled) || (enabled == "false" && u.Enabled) {
			continue
		}
		filtered = append(filtered, u)
	}
	writeJSON(w, http.StatusOK, AdminUsersResponse{Users: filtered, Total: len(filtered)})
}

func userMatches(u keycloak.IdentityUser, needle string) bool {
	for _, f := range []string{u.Username, u.Email, u.FirstName, u.LastName} {
		if strings.Contains(strings.ToLower(f), needle) {
			return true
		}
	}
	return false
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// handleAdminGetUser serves GET /api/v1/admin/users/{id}.
//
//	@Summary		Get a user (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			id	path		string	true	"Keycloak user id"
//	@Success		200	{object}	keycloak.IdentityUser
//	@Failure		404	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/users/{id} [get]
func (h *Handler) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	h.respondUser(w, r, r.PathValue("id"))
}

func (h *Handler) respondUser(w http.ResponseWriter, r *http.Request, id string) {
	u, err := h.identity.GetUser(r.Context(), id)
	if err != nil {
		writeKeycloakError(w, err, "user")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// handleAdminPatchUser serves PATCH /api/v1/admin/users/{id}.
//
//	@Summary		Enable or disable a user (admin)
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Keycloak user id"
//	@Param			body	body		AdminUserPatch	true	"Fields to change"
//	@Success		200		{object}	keycloak.IdentityUser
//	@Failure		400		{object}	AdminError
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/users/{id} [patch]
func (h *Handler) handleAdminPatchUser(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	var body AdminUserPatch
	if !decodeBody(w, r, &body) {
		return
	}
	id := r.PathValue("id")
	if body.Enabled != nil {
		if err := h.identity.SetUserEnabled(r.Context(), id, *body.Enabled); err != nil {
			writeKeycloakError(w, err, "user")
			return
		}
	}
	h.respondUser(w, r, id)
}

// handleAdminDeleteUser serves DELETE /api/v1/admin/users/{id}.
//
//	@Summary		Delete a user (admin)
//	@Description	Permanently deletes the realm user together with their group memberships, role mappings and sessions. Admins cannot delete their own account.
//	@Tags			admin
//	@Produce		json
//	@Param			id	path	string	true	"Keycloak user id"
//	@Success		204
//	@Failure		403	{object}	AdminError	"Cannot delete your own account"
//	@Failure		404	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/users/{id} [delete]
func (h *Handler) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}
	if h.identity == nil {
		writeAdminError(w, http.StatusNotImplemented,
			"Keycloak admin client not configured (set webapi.keycloak.adminSecretName)")
		return
	}
	id := r.PathValue("id")
	if claims.Subject != "" && claims.Subject == id {
		writeAdminError(w, http.StatusForbidden, "you cannot delete your own account")
		return
	}
	if err := h.identity.DeleteUser(r.Context(), id); err != nil {
		writeKeycloakError(w, err, "user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAdminUserGroup serves PUT/DELETE /api/v1/admin/users/{id}/groups/{groupId}.
//
//	@Summary		Add or remove a user's group membership (admin)
//	@Description	PUT adds the user to the group; DELETE removes them. Both are idempotent.
//	@Tags			admin
//	@Produce		json
//	@Param			id		path		string	true	"Keycloak user id"
//	@Param			groupId	path		string	true	"Keycloak group id"
//	@Success		200		{object}	keycloak.IdentityUser
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/users/{id}/groups/{groupId} [put]
//	@Router			/admin/users/{id}/groups/{groupId} [delete]
func (h *Handler) handleAdminUserGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	id, groupID := r.PathValue("id"), r.PathValue("groupId")
	var err error
	if r.Method == http.MethodPut {
		err = h.identity.AddUserToGroupByID(r.Context(), id, groupID)
	} else {
		err = h.identity.RemoveUserFromGroup(r.Context(), id, groupID)
	}
	if err != nil {
		writeKeycloakError(w, err, "user or group")
		return
	}
	h.respondUser(w, r, id)
}

// handleAdminUserRole serves PUT/DELETE /api/v1/admin/users/{id}/roles/{role}.
//
//	@Summary		Assign or unassign a direct realm role (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			id		path		string	true	"Keycloak user id"
//	@Param			role	path		string	true	"Realm role name"
//	@Success		200		{object}	keycloak.IdentityUser
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/users/{id}/roles/{role} [put]
//	@Router			/admin/users/{id}/roles/{role} [delete]
func (h *Handler) handleAdminUserRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	id, role := r.PathValue("id"), r.PathValue("role")
	var err error
	if r.Method == http.MethodPut {
		err = h.identity.AssignRealmRole(r.Context(), id, role)
	} else {
		err = h.identity.UnassignRealmRole(r.Context(), id, role)
	}
	if err != nil {
		writeKeycloakError(w, err, "user or role")
		return
	}
	h.respondUser(w, r, id)
}

// handleAdminBulkUsers serves POST /api/v1/admin/users/bulk.
//
//	@Summary		Apply one change to many users (admin)
//	@Description	Actions: addToGroup / removeFromGroup (need groupId), assignRole / unassignRole (need role), enable, disable. Users that fail are skipped and counted out of `updated`.
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		BulkUsersRequest	true	"Users and action"
//	@Success		200		{object}	BulkUsersResponse
//	@Failure		400		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/users/bulk [post]
func (h *Handler) handleAdminBulkUsers(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	var body BulkUsersRequest
	if !decodeBody(w, r, &body) {
		return
	}
	var apply func(ctx context.Context, id string) error
	switch body.Action {
	case "addToGroup", "removeFromGroup":
		if body.GroupID == "" {
			writeAdminError(w, http.StatusBadRequest, "groupId required")
			return
		}
		if body.Action == "addToGroup" {
			apply = func(ctx context.Context, id string) error {
				return h.identity.AddUserToGroupByID(ctx, id, body.GroupID)
			}
		} else {
			apply = func(ctx context.Context, id string) error {
				return h.identity.RemoveUserFromGroup(ctx, id, body.GroupID)
			}
		}
	case "assignRole", "unassignRole":
		if body.Role == "" {
			writeAdminError(w, http.StatusBadRequest, "role required")
			return
		}
		if body.Action == "assignRole" {
			apply = func(ctx context.Context, id string) error {
				return h.identity.AssignRealmRole(ctx, id, body.Role)
			}
		} else {
			apply = func(ctx context.Context, id string) error {
				return h.identity.UnassignRealmRole(ctx, id, body.Role)
			}
		}
	case "enable", "disable":
		enabled := body.Action == "enable"
		apply = func(ctx context.Context, id string) error {
			return h.identity.SetUserEnabled(ctx, id, enabled)
		}
	default:
		writeAdminError(w, http.StatusBadRequest, "unknown action")
		return
	}

	updated := 0
	for _, id := range body.UserIDs {
		if err := apply(r.Context(), id); err != nil {
			log.Error(err, "Bulk user update failed for one user", "user", id, "action", body.Action)
			continue
		}
		updated++
	}
	writeJSON(w, http.StatusOK, BulkUsersResponse{Updated: updated})
}

// --- groups ----------------------------------------------------------------

// handleAdminListGroups serves GET /api/v1/admin/groups.
//
//	@Summary		List groups (admin)
//	@Tags			admin
//	@Produce		json
//	@Success		200	{array}		keycloak.IdentityGroup
//	@Failure		501	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/groups [get]
func (h *Handler) handleAdminListGroups(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	groups, err := h.identity.ListGroups(r.Context())
	if err != nil {
		writeKeycloakError(w, err, "groups")
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// handleAdminGetGroup serves GET /api/v1/admin/groups/{id}.
//
//	@Summary		Get a group (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			id	path		string	true	"Keycloak group id"
//	@Success		200	{object}	keycloak.IdentityGroup
//	@Failure		404	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/groups/{id} [get]
func (h *Handler) handleAdminGetGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	h.respondGroup(w, r, r.PathValue("id"))
}

func (h *Handler) respondGroup(w http.ResponseWriter, r *http.Request, id string) {
	g, err := h.identity.GetGroup(r.Context(), id)
	if err != nil {
		writeKeycloakError(w, err, "group")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// handleAdminCreateGroup serves POST /api/v1/admin/groups.
//
//	@Summary		Create a top-level group (admin)
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		AdminGroupInput	true	"Group name (lowercase, digits, dashes) and description"
//	@Success		201		{object}	keycloak.IdentityGroup
//	@Failure		400		{object}	AdminError
//	@Failure		409		{object}	AdminError	"Group already exists"
//	@Security		BearerAuth
//	@Router			/admin/groups [post]
func (h *Handler) handleAdminCreateGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	var body AdminGroupInput
	if !decodeBody(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if !identifierRe.MatchString(name) {
		writeAdminError(w, http.StatusBadRequest, "name must be lowercase letters, digits, dots, dashes or underscores")
		return
	}
	g, err := h.identity.CreateGroup(r.Context(), name, body.Description)
	if err != nil {
		writeKeycloakError(w, err, "group")
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

// handleAdminPatchGroup serves PATCH /api/v1/admin/groups/{id}.
//
//	@Summary		Update a group's description (admin)
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string			true	"Keycloak group id"
//	@Param			body	body		AdminGroupInput	true	"New description"
//	@Success		200		{object}	keycloak.IdentityGroup
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/groups/{id} [patch]
func (h *Handler) handleAdminPatchGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	var body AdminGroupInput
	if !decodeBody(w, r, &body) {
		return
	}
	id := r.PathValue("id")
	if err := h.identity.UpdateGroupDescription(r.Context(), id, body.Description); err != nil {
		writeKeycloakError(w, err, "group")
		return
	}
	h.respondGroup(w, r, id)
}

// handleAdminDeleteGroup serves DELETE /api/v1/admin/groups/{id}.
//
//	@Summary		Delete a group (admin)
//	@Description	Refused with 409 while any landing-page service lists the group under spec.auth.groups in its NebariApp; remove it from the CR first.
//	@Tags			admin
//	@Produce		json
//	@Param			id	path	string	true	"Keycloak group id"
//	@Success		204
//	@Failure		404	{object}	AdminError
//	@Failure		409	{object}	AdminError	"Group is referenced by a service gate"
//	@Security		BearerAuth
//	@Router			/admin/groups/{id} [delete]
func (h *Handler) handleAdminDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	g, err := h.identity.GetGroup(r.Context(), id)
	if err != nil {
		writeKeycloakError(w, err, "group")
		return
	}
	if refs := h.servicesReferencingGroup(g.Name); len(refs) > 0 {
		writeJSON(w, http.StatusConflict, AdminError{
			Error:    "group is referenced by NebariApp.requiredGroups",
			Services: refs,
		})
		return
	}
	if err := h.identity.DeleteGroup(r.Context(), id); err != nil {
		writeKeycloakError(w, err, "group")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) servicesReferencingGroup(name string) []string {
	var refs []string
	for _, s := range h.cache.GetAll() {
		if contains(s.RequiredGroups, name) {
			refs = append(refs, s.UID)
		}
	}
	return refs
}

// handleAdminGroupMember serves PUT/DELETE /api/v1/admin/groups/{id}/members/{userId}.
//
//	@Summary		Add or remove a group member (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			id		path		string	true	"Keycloak group id"
//	@Param			userId	path		string	true	"Keycloak user id"
//	@Success		200		{object}	keycloak.IdentityGroup
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/groups/{id}/members/{userId} [put]
//	@Router			/admin/groups/{id}/members/{userId} [delete]
func (h *Handler) handleAdminGroupMember(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	id, userID := r.PathValue("id"), r.PathValue("userId")
	var err error
	if r.Method == http.MethodPut {
		err = h.identity.AddUserToGroupByID(r.Context(), userID, id)
	} else {
		err = h.identity.RemoveUserFromGroup(r.Context(), userID, id)
	}
	if err != nil {
		writeKeycloakError(w, err, "group or user")
		return
	}
	h.respondGroup(w, r, id)
}

// handleAdminGroupRole serves PUT/DELETE /api/v1/admin/groups/{id}/roles/{role}.
//
//	@Summary		Map or unmap a realm role on a group (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			id		path		string	true	"Keycloak group id"
//	@Param			role	path		string	true	"Realm role name"
//	@Success		200		{object}	keycloak.IdentityGroup
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/groups/{id}/roles/{role} [put]
//	@Router			/admin/groups/{id}/roles/{role} [delete]
func (h *Handler) handleAdminGroupRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	id, role := r.PathValue("id"), r.PathValue("role")
	var err error
	if r.Method == http.MethodPut {
		err = h.identity.AddRealmRoleToGroup(r.Context(), id, role)
	} else {
		err = h.identity.RemoveRealmRoleFromGroup(r.Context(), id, role)
	}
	if err != nil {
		writeKeycloakError(w, err, "group or role")
		return
	}
	h.respondGroup(w, r, id)
}

// --- roles -----------------------------------------------------------------

// handleAdminListRoles serves GET /api/v1/admin/roles.
//
//	@Summary		List realm roles (admin)
//	@Tags			admin
//	@Produce		json
//	@Success		200	{array}		keycloak.IdentityRole
//	@Failure		501	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/roles [get]
func (h *Handler) handleAdminListRoles(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	roles, err := h.identity.ListRoles(r.Context())
	if err != nil {
		writeKeycloakError(w, err, "roles")
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

// handleAdminCreateRole serves POST /api/v1/admin/roles.
//
//	@Summary		Create a realm role (admin)
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			body	body		AdminRoleInput	true	"Role name and description"
//	@Success		201		{object}	keycloak.IdentityRole
//	@Failure		400		{object}	AdminError
//	@Failure		409		{object}	AdminError	"Role already exists"
//	@Security		BearerAuth
//	@Router			/admin/roles [post]
func (h *Handler) handleAdminCreateRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	var body AdminRoleInput
	if !decodeBody(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if !identifierRe.MatchString(name) {
		writeAdminError(w, http.StatusBadRequest, "name must be lowercase letters, digits, dots, dashes or underscores")
		return
	}
	role, err := h.identity.CreateRole(r.Context(), name, body.Description)
	if err != nil {
		writeKeycloakError(w, err, "role")
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (h *Handler) findRole(ctx context.Context, name string) (*keycloak.IdentityRole, error) {
	roles, err := h.identity.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		if roles[i].Name == name {
			return &roles[i], nil
		}
	}
	return nil, errNotFound
}

var errNotFound = errors.New("not found")

// handleAdminPatchRole serves PATCH /api/v1/admin/roles/{name}.
//
//	@Summary		Update a realm role's description (admin)
//	@Tags			admin
//	@Accept			json
//	@Produce		json
//	@Param			name	path		string			true	"Realm role name"
//	@Param			body	body		AdminRoleInput	true	"New description"
//	@Success		200		{object}	keycloak.IdentityRole
//	@Failure		403		{object}	AdminError	"Built-in roles cannot be edited"
//	@Failure		404		{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/roles/{name} [patch]
func (h *Handler) handleAdminPatchRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	var body AdminRoleInput
	if !decodeBody(w, r, &body) {
		return
	}
	name := r.PathValue("name")
	role, err := h.findRole(r.Context(), name)
	if errors.Is(err, errNotFound) {
		writeAdminError(w, http.StatusNotFound, "role not found")
		return
	}
	if err != nil {
		writeKeycloakError(w, err, "role")
		return
	}
	if role.BuiltIn {
		writeAdminError(w, http.StatusForbidden, "built-in roles cannot be edited")
		return
	}
	if err := h.identity.UpdateRoleDescription(r.Context(), name, body.Description); err != nil {
		writeKeycloakError(w, err, "role")
		return
	}
	role.Description = body.Description
	writeJSON(w, http.StatusOK, role)
}

// handleAdminDeleteRole serves DELETE /api/v1/admin/roles/{name}.
//
//	@Summary		Delete a realm role (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			name	path	string	true	"Realm role name"
//	@Success		204
//	@Failure		403	{object}	AdminError	"Built-in roles cannot be deleted"
//	@Failure		404	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/roles/{name} [delete]
func (h *Handler) handleAdminDeleteRole(w http.ResponseWriter, r *http.Request) {
	if !h.requireIdentityAdmin(w, r) {
		return
	}
	name := r.PathValue("name")
	role, err := h.findRole(r.Context(), name)
	if errors.Is(err, errNotFound) {
		writeAdminError(w, http.StatusNotFound, "role not found")
		return
	}
	if err != nil {
		writeKeycloakError(w, err, "role")
		return
	}
	if role.BuiltIn {
		writeAdminError(w, http.StatusForbidden, "built-in roles cannot be deleted")
		return
	}
	if err := h.identity.DeleteRole(r.Context(), name); err != nil {
		writeKeycloakError(w, err, "role")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- services --------------------------------------------------------------

func toAdminService(s *cache.ServiceInfo) AdminService {
	vis := s.Visibility
	if vis == "" {
		vis = "private"
	}
	groups := s.RequiredGroups
	if groups == nil {
		groups = []string{}
	}
	return AdminService{
		ID:             s.UID,
		Name:           s.Name,
		DisplayName:    s.DisplayName,
		Namespace:      s.Namespace,
		Category:       s.Category,
		URL:            s.URL,
		Visibility:     vis,
		RequiredGroups: groups,
	}
}

// handleAdminListServices serves GET /api/v1/admin/services.
//
//	@Summary		List service access gates (admin)
//	@Description	Every landing-page service with the gate derived from its NebariApp spec.auth (visibility + requiredGroups). Read-only: the CR is the source of truth. Does not require a Keycloak admin client.
//	@Tags			admin
//	@Produce		json
//	@Success		200	{array}		AdminService
//	@Failure		403	{string}	string	"Forbidden: admin group required"
//	@Security		BearerAuth
//	@Router			/admin/services [get]
func (h *Handler) handleAdminListServices(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	all := h.cache.GetAll()
	out := make([]AdminService, 0, len(all))
	for _, s := range all {
		out = append(out, toAdminService(s))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleAdminGetService serves GET /api/v1/admin/services/{id}.
//
//	@Summary		Get a service access gate (admin)
//	@Tags			admin
//	@Produce		json
//	@Param			id	path		string	true	"Service UID"
//	@Success		200	{object}	AdminService
//	@Failure		404	{object}	AdminError
//	@Security		BearerAuth
//	@Router			/admin/services/{id} [get]
func (h *Handler) handleAdminGetService(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	s := h.cache.Get(r.PathValue("id"))
	if s == nil {
		writeAdminError(w, http.StatusNotFound, "service not found")
		return
	}
	writeJSON(w, http.StatusOK, toAdminService(s))
}
