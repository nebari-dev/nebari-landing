// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package keycloak

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
)

// IdentityUser is the admin-area view of a realm user.
type IdentityUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
	// LastSignInAt is not exposed by the Keycloak user representation (it lives
	// in the events store), so it is always null for now.
	LastSignInAt *string `json:"lastSignInAt"`
	// Groups holds the ids of the groups the user is a direct member of.
	Groups []string `json:"groups"`
	// Roles holds realm role names mapped directly to the user.
	Roles []string `json:"roles"`
}

// IdentityGroup is the admin-area view of a top-level realm group.
type IdentityGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	// Description is stored as the "description" group attribute; Keycloak
	// groups have no first-class description field.
	Description string   `json:"description"`
	Roles       []string `json:"roles"`
	CreatedAt   string   `json:"createdAt"`
}

// IdentityRole is the admin-area view of a realm role.
type IdentityRole struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Composite   bool   `json:"composite"`
	// BuiltIn marks Keycloak-managed roles that the UI must not edit.
	BuiltIn bool `json:"builtIn"`
}

const groupDescriptionAttr = "description"

// StatusCode extracts the HTTP status Keycloak returned for err, or 0 when
// err did not originate from the admin API.
func StatusCode(err error) int {
	var apiErr *gocloak.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	var apiVal gocloak.APIError
	if errors.As(err, &apiVal) {
		return apiVal.Code
	}
	return 0
}

// IsNotFound reports whether err is a Keycloak 404.
func IsNotFound(err error) bool { return StatusCode(err) == 404 }

// IsConflict reports whether err is a Keycloak 409.
func IsConflict(err error) bool { return StatusCode(err) == 409 }

func isBuiltInRole(name string) bool {
	return name == "offline_access" || name == "uma_authorization" || strings.HasPrefix(name, "default-roles-")
}

func millisToISO(ms *int64) string {
	if ms == nil {
		return ""
	}
	return time.UnixMilli(*ms).UTC().Format(time.RFC3339)
}

// session bundles an authenticated gocloak client for one admin operation.
type session struct {
	kc    *gocloak.GoCloak
	token string
	realm string
}

func (c *Client) session(ctx context.Context) (*session, error) {
	kc, token, err := c.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	return &session{kc: kc, token: token.AccessToken, realm: c.cfg.Realm}, nil
}

// --- users ---------------------------------------------------------------

// ListUsers returns every user in the realm with direct group ids and realm
// role names resolved. It issues two extra admin calls per user, which is
// fine for the realm sizes Nebari targets; switch to per-group member
// listing if that ever becomes a problem.
func (c *Client) ListUsers(ctx context.Context) ([]IdentityUser, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	const pageSize = 200
	var out []IdentityUser
	for first := 0; ; first += pageSize {
		users, err := s.kc.GetUsers(ctx, s.token, s.realm, gocloak.GetUsersParams{
			First: gocloak.IntP(first),
			Max:   gocloak.IntP(pageSize),
		})
		if err != nil {
			return nil, fmt.Errorf("listing users in realm %q: %w", s.realm, err)
		}
		for _, u := range users {
			iu, err := s.hydrateUser(ctx, u)
			if err != nil {
				return nil, err
			}
			out = append(out, iu)
		}
		if len(users) < pageSize {
			break
		}
	}
	if out == nil {
		out = []IdentityUser{}
	}
	return out, nil
}

// GetUser returns one user with memberships resolved.
func (c *Client) GetUser(ctx context.Context, id string) (*IdentityUser, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	u, err := s.kc.GetUserByID(ctx, s.token, s.realm, id)
	if err != nil {
		return nil, fmt.Errorf("getting user %q: %w", id, err)
	}
	iu, err := s.hydrateUser(ctx, u)
	if err != nil {
		return nil, err
	}
	return &iu, nil
}

func (s *session) hydrateUser(ctx context.Context, u *gocloak.User) (IdentityUser, error) {
	id := gocloak.PString(u.ID)
	iu := IdentityUser{
		ID:        id,
		Username:  gocloak.PString(u.Username),
		Email:     gocloak.PString(u.Email),
		FirstName: gocloak.PString(u.FirstName),
		LastName:  gocloak.PString(u.LastName),
		Enabled:   gocloak.PBool(u.Enabled),
		CreatedAt: millisToISO(u.CreatedTimestamp),
		Groups:    []string{},
		Roles:     []string{},
	}
	groups, err := s.kc.GetUserGroups(ctx, s.token, s.realm, id, gocloak.GetGroupsParams{
		BriefRepresentation: gocloak.BoolP(true),
	})
	if err != nil {
		return iu, fmt.Errorf("listing groups for user %q: %w", id, err)
	}
	for _, g := range groups {
		iu.Groups = append(iu.Groups, gocloak.PString(g.ID))
	}
	roles, err := s.kc.GetRealmRolesByUserID(ctx, s.token, s.realm, id)
	if err != nil {
		return iu, fmt.Errorf("listing roles for user %q: %w", id, err)
	}
	for _, r := range roles {
		iu.Roles = append(iu.Roles, gocloak.PString(r.Name))
	}
	return iu, nil
}

// SetUserEnabled toggles the account's enabled flag.
func (c *Client) SetUserEnabled(ctx context.Context, id string, enabled bool) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	u, err := s.kc.GetUserByID(ctx, s.token, s.realm, id)
	if err != nil {
		return fmt.Errorf("getting user %q: %w", id, err)
	}
	u.Enabled = gocloak.BoolP(enabled)
	if err := s.kc.UpdateUser(ctx, s.token, s.realm, *u); err != nil {
		return fmt.Errorf("updating user %q: %w", id, err)
	}
	return nil
}

// AddUserToGroupByID adds an existing user to an existing group by ids.
func (c *Client) AddUserToGroupByID(ctx context.Context, userID, groupID string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	if err := s.kc.AddUserToGroup(ctx, s.token, s.realm, userID, groupID); err != nil {
		return fmt.Errorf("adding user %q to group %q: %w", userID, groupID, err)
	}
	return nil
}

// RemoveUserFromGroup removes a user from a group by ids.
func (c *Client) RemoveUserFromGroup(ctx context.Context, userID, groupID string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	if err := s.kc.DeleteUserFromGroup(ctx, s.token, s.realm, userID, groupID); err != nil {
		return fmt.Errorf("removing user %q from group %q: %w", userID, groupID, err)
	}
	return nil
}

// AssignRealmRole maps a realm role directly to a user.
func (c *Client) AssignRealmRole(ctx context.Context, userID, roleName string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	role, err := s.kc.GetRealmRole(ctx, s.token, s.realm, roleName)
	if err != nil {
		return fmt.Errorf("getting role %q: %w", roleName, err)
	}
	if err := s.kc.AddRealmRoleToUser(ctx, s.token, s.realm, userID, []gocloak.Role{*role}); err != nil {
		return fmt.Errorf("assigning role %q to user %q: %w", roleName, userID, err)
	}
	return nil
}

// UnassignRealmRole removes a direct realm role mapping from a user.
func (c *Client) UnassignRealmRole(ctx context.Context, userID, roleName string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	role, err := s.kc.GetRealmRole(ctx, s.token, s.realm, roleName)
	if err != nil {
		return fmt.Errorf("getting role %q: %w", roleName, err)
	}
	if err := s.kc.DeleteRealmRoleFromUser(ctx, s.token, s.realm, userID, []gocloak.Role{*role}); err != nil {
		return fmt.Errorf("removing role %q from user %q: %w", roleName, userID, err)
	}
	return nil
}

// --- groups --------------------------------------------------------------

// ListGroups returns the realm's top-level groups with their realm roles.
func (c *Client) ListGroups(ctx context.Context) ([]IdentityGroup, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	groups, err := s.kc.GetGroups(ctx, s.token, s.realm, gocloak.GetGroupsParams{
		Max: gocloak.IntP(1000),
	})
	if err != nil {
		return nil, fmt.Errorf("listing groups in realm %q: %w", s.realm, err)
	}
	out := make([]IdentityGroup, 0, len(groups))
	for _, g := range groups {
		ig, err := s.hydrateGroup(ctx, g)
		if err != nil {
			return nil, err
		}
		out = append(out, ig)
	}
	return out, nil
}

// GetGroup returns one group with its realm roles.
func (c *Client) GetGroup(ctx context.Context, id string) (*IdentityGroup, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	g, err := s.kc.GetGroup(ctx, s.token, s.realm, id)
	if err != nil {
		return nil, fmt.Errorf("getting group %q: %w", id, err)
	}
	ig, err := s.hydrateGroup(ctx, g)
	if err != nil {
		return nil, err
	}
	return &ig, nil
}

func (s *session) hydrateGroup(ctx context.Context, g *gocloak.Group) (IdentityGroup, error) {
	id := gocloak.PString(g.ID)
	ig := IdentityGroup{
		ID:    id,
		Name:  gocloak.PString(g.Name),
		Path:  gocloak.PString(g.Path),
		Roles: []string{},
	}
	if g.Attributes != nil {
		if v, ok := (*g.Attributes)[groupDescriptionAttr]; ok && len(v) > 0 {
			ig.Description = v[0]
		}
	}
	roles, err := s.kc.GetRealmRolesByGroupID(ctx, s.token, s.realm, id)
	if err != nil {
		return ig, fmt.Errorf("listing roles for group %q: %w", id, err)
	}
	for _, r := range roles {
		ig.Roles = append(ig.Roles, gocloak.PString(r.Name))
	}
	return ig, nil
}

// CreateGroup creates a top-level group. Keycloak answers 409 when the name
// is taken; callers can detect that with IsConflict.
func (c *Client) CreateGroup(ctx context.Context, name, description string) (*IdentityGroup, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	attrs := map[string][]string{}
	if description != "" {
		attrs[groupDescriptionAttr] = []string{description}
	}
	id, err := s.kc.CreateGroup(ctx, s.token, s.realm, gocloak.Group{
		Name:       gocloak.StringP(name),
		Attributes: &attrs,
	})
	if err != nil {
		return nil, fmt.Errorf("creating group %q: %w", name, err)
	}
	log.Info("Created Keycloak group", "realm", s.realm, "group", name, "id", id)
	g, err := s.kc.GetGroup(ctx, s.token, s.realm, id)
	if err != nil {
		return nil, fmt.Errorf("reading back group %q: %w", name, err)
	}
	ig, err := s.hydrateGroup(ctx, g)
	if err != nil {
		return nil, err
	}
	return &ig, nil
}

// UpdateGroupDescription rewrites the group's description attribute.
func (c *Client) UpdateGroupDescription(ctx context.Context, id, description string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	g, err := s.kc.GetGroup(ctx, s.token, s.realm, id)
	if err != nil {
		return fmt.Errorf("getting group %q: %w", id, err)
	}
	attrs := map[string][]string{}
	if g.Attributes != nil {
		attrs = *g.Attributes
	}
	attrs[groupDescriptionAttr] = []string{description}
	g.Attributes = &attrs
	if err := s.kc.UpdateGroup(ctx, s.token, s.realm, *g); err != nil {
		return fmt.Errorf("updating group %q: %w", id, err)
	}
	return nil
}

// DeleteGroup removes a group; members simply lose the membership.
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	if err := s.kc.DeleteGroup(ctx, s.token, s.realm, id); err != nil {
		return fmt.Errorf("deleting group %q: %w", id, err)
	}
	return nil
}

// AddRealmRoleToGroup maps a realm role to a group.
func (c *Client) AddRealmRoleToGroup(ctx context.Context, groupID, roleName string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	role, err := s.kc.GetRealmRole(ctx, s.token, s.realm, roleName)
	if err != nil {
		return fmt.Errorf("getting role %q: %w", roleName, err)
	}
	if err := s.kc.AddRealmRoleToGroup(ctx, s.token, s.realm, groupID, []gocloak.Role{*role}); err != nil {
		return fmt.Errorf("adding role %q to group %q: %w", roleName, groupID, err)
	}
	return nil
}

// RemoveRealmRoleFromGroup unmaps a realm role from a group.
func (c *Client) RemoveRealmRoleFromGroup(ctx context.Context, groupID, roleName string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	role, err := s.kc.GetRealmRole(ctx, s.token, s.realm, roleName)
	if err != nil {
		return fmt.Errorf("getting role %q: %w", roleName, err)
	}
	if err := s.kc.DeleteRealmRoleFromGroup(ctx, s.token, s.realm, groupID, []gocloak.Role{*role}); err != nil {
		return fmt.Errorf("removing role %q from group %q: %w", roleName, groupID, err)
	}
	return nil
}

// --- roles ---------------------------------------------------------------

// ListRoles returns every realm role.
func (c *Client) ListRoles(ctx context.Context) ([]IdentityRole, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := s.kc.GetRealmRoles(ctx, s.token, s.realm, gocloak.GetRoleParams{Max: gocloak.IntP(1000)})
	if err != nil {
		return nil, fmt.Errorf("listing roles in realm %q: %w", s.realm, err)
	}
	out := make([]IdentityRole, 0, len(roles))
	for _, r := range roles {
		out = append(out, toIdentityRole(r))
	}
	return out, nil
}

func toIdentityRole(r *gocloak.Role) IdentityRole {
	name := gocloak.PString(r.Name)
	desc := gocloak.PString(r.Description)
	// Built-in roles carry i18n placeholders like "${role_offline-access}".
	if strings.HasPrefix(desc, "${") {
		desc = ""
	}
	return IdentityRole{
		Name:        name,
		Description: desc,
		Composite:   gocloak.PBool(r.Composite),
		BuiltIn:     isBuiltInRole(name),
	}
}

// CreateRole creates a realm role.
func (c *Client) CreateRole(ctx context.Context, name, description string) (*IdentityRole, error) {
	s, err := c.session(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.kc.CreateRealmRole(ctx, s.token, s.realm, gocloak.Role{
		Name:        gocloak.StringP(name),
		Description: gocloak.StringP(description),
	}); err != nil {
		return nil, fmt.Errorf("creating role %q: %w", name, err)
	}
	r, err := s.kc.GetRealmRole(ctx, s.token, s.realm, name)
	if err != nil {
		return nil, fmt.Errorf("reading back role %q: %w", name, err)
	}
	ir := toIdentityRole(r)
	return &ir, nil
}

// UpdateRoleDescription rewrites a realm role's description.
func (c *Client) UpdateRoleDescription(ctx context.Context, name, description string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	r, err := s.kc.GetRealmRole(ctx, s.token, s.realm, name)
	if err != nil {
		return fmt.Errorf("getting role %q: %w", name, err)
	}
	r.Description = gocloak.StringP(description)
	if err := s.kc.UpdateRealmRole(ctx, s.token, s.realm, name, *r); err != nil {
		return fmt.Errorf("updating role %q: %w", name, err)
	}
	return nil
}

// DeleteRole removes a realm role; Keycloak drops all its mappings.
func (c *Client) DeleteRole(ctx context.Context, name string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	if err := s.kc.DeleteRealmRole(ctx, s.token, s.realm, name); err != nil {
		return fmt.Errorf("deleting role %q: %w", name, err)
	}
	return nil
}

// DeleteUser permanently removes a realm user; Keycloak drops all of their
// group memberships, role mappings and sessions.
func (c *Client) DeleteUser(ctx context.Context, id string) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	if err := s.kc.DeleteUser(ctx, s.token, s.realm, id); err != nil {
		return fmt.Errorf("deleting user %q: %w", id, err)
	}
	log.Info("Deleted Keycloak user", "realm", s.realm, "id", id)
	return nil
}
