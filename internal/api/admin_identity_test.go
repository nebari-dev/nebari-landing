// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/golang-jwt/jwt/v5"

	"github.com/nebari-dev/nebari-landing/internal/app"
	"github.com/nebari-dev/nebari-landing/internal/auth"
	"github.com/nebari-dev/nebari-landing/internal/cache"
	webkeycloak "github.com/nebari-dev/nebari-landing/internal/keycloak"
	"github.com/nebari-dev/nebari-landing/internal/packs"
)

// fakeIdentity is an in-memory IdentityAdmin that mimics Keycloak's error
// codes closely enough for the handler tests.
type fakeIdentity struct {
	users  map[string]*webkeycloak.IdentityUser
	groups map[string]*webkeycloak.IdentityGroup
	roles  map[string]*webkeycloak.IdentityRole
}

func notFound() error { return &gocloak.APIError{Code: 404, Message: "not found"} }
func conflict() error { return &gocloak.APIError{Code: 409, Message: "exists"} }

func newFakeIdentity() *fakeIdentity {
	return &fakeIdentity{
		users: map[string]*webkeycloak.IdentityUser{
			"u-alice": {ID: "u-alice", Username: "alice", Email: "alice@example.com", FirstName: "Alice", LastName: "A", Enabled: true, Groups: []string{"g-admin", "g-users"}, Roles: []string{}},
			"u-bob":   {ID: "u-bob", Username: "bob", Email: "bob@example.com", FirstName: "Bob", LastName: "B", Enabled: true, Groups: []string{"g-users"}, Roles: []string{"viewer"}},
			"u-eve":   {ID: "u-eve", Username: "eve", Email: "eve@example.com", FirstName: "Eve", LastName: "E", Enabled: false, Groups: []string{}, Roles: []string{}},
		},
		groups: map[string]*webkeycloak.IdentityGroup{
			"g-admin": {ID: "g-admin", Name: "admin", Path: "/admin", Roles: []string{"admin"}},
			"g-users": {ID: "g-users", Name: "users", Path: "/users", Roles: []string{}},
		},
		roles: map[string]*webkeycloak.IdentityRole{
			"admin":                {Name: "admin"},
			"viewer":               {Name: "viewer"},
			"offline_access":       {Name: "offline_access", BuiltIn: true},
			"default-roles-nebari": {Name: "default-roles-nebari", BuiltIn: true, Composite: true},
		},
	}
}

func (f *fakeIdentity) ListUsers(context.Context) ([]webkeycloak.IdentityUser, error) {
	out := []webkeycloak.IdentityUser{}
	for _, id := range []string{"u-alice", "u-bob", "u-eve"} {
		if u, ok := f.users[id]; ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (f *fakeIdentity) GetUser(_ context.Context, id string) (*webkeycloak.IdentityUser, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, notFound()
	}
	cp := *u
	return &cp, nil
}

func (f *fakeIdentity) DeleteUser(_ context.Context, id string) error {
	if _, ok := f.users[id]; !ok {
		return notFound()
	}
	delete(f.users, id)
	return nil
}

func (f *fakeIdentity) SetUserEnabled(_ context.Context, id string, enabled bool) error {
	u, ok := f.users[id]
	if !ok {
		return notFound()
	}
	u.Enabled = enabled
	return nil
}

func (f *fakeIdentity) AddUserToGroupByID(_ context.Context, userID, groupID string) error {
	u, ok := f.users[userID]
	if !ok {
		return notFound()
	}
	if _, ok := f.groups[groupID]; !ok {
		return notFound()
	}
	if !contains(u.Groups, groupID) {
		u.Groups = append(u.Groups, groupID)
	}
	return nil
}

func (f *fakeIdentity) RemoveUserFromGroup(_ context.Context, userID, groupID string) error {
	u, ok := f.users[userID]
	if !ok {
		return notFound()
	}
	out := u.Groups[:0]
	for _, g := range u.Groups {
		if g != groupID {
			out = append(out, g)
		}
	}
	u.Groups = out
	return nil
}

func (f *fakeIdentity) AssignRealmRole(_ context.Context, userID, role string) error {
	u, ok := f.users[userID]
	if !ok {
		return notFound()
	}
	if _, ok := f.roles[role]; !ok {
		return notFound()
	}
	if !contains(u.Roles, role) {
		u.Roles = append(u.Roles, role)
	}
	return nil
}

func (f *fakeIdentity) UnassignRealmRole(_ context.Context, userID, role string) error {
	u, ok := f.users[userID]
	if !ok {
		return notFound()
	}
	out := u.Roles[:0]
	for _, r := range u.Roles {
		if r != role {
			out = append(out, r)
		}
	}
	u.Roles = out
	return nil
}

func (f *fakeIdentity) ListGroups(context.Context) ([]webkeycloak.IdentityGroup, error) {
	out := []webkeycloak.IdentityGroup{}
	for _, g := range f.groups {
		out = append(out, *g)
	}
	return out, nil
}

func (f *fakeIdentity) GetGroup(_ context.Context, id string) (*webkeycloak.IdentityGroup, error) {
	g, ok := f.groups[id]
	if !ok {
		return nil, notFound()
	}
	cp := *g
	return &cp, nil
}

func (f *fakeIdentity) CreateGroup(_ context.Context, name, description string) (*webkeycloak.IdentityGroup, error) {
	for _, g := range f.groups {
		if g.Name == name {
			return nil, conflict()
		}
	}
	g := &webkeycloak.IdentityGroup{ID: "g-" + name, Name: name, Path: "/" + name, Description: description, Roles: []string{}}
	f.groups[g.ID] = g
	return g, nil
}

func (f *fakeIdentity) UpdateGroupDescription(_ context.Context, id, description string) error {
	g, ok := f.groups[id]
	if !ok {
		return notFound()
	}
	g.Description = description
	return nil
}

func (f *fakeIdentity) DeleteGroup(_ context.Context, id string) error {
	if _, ok := f.groups[id]; !ok {
		return notFound()
	}
	delete(f.groups, id)
	return nil
}

func (f *fakeIdentity) AddRealmRoleToGroup(_ context.Context, groupID, role string) error {
	g, ok := f.groups[groupID]
	if !ok {
		return notFound()
	}
	if !contains(g.Roles, role) {
		g.Roles = append(g.Roles, role)
	}
	return nil
}

func (f *fakeIdentity) RemoveRealmRoleFromGroup(_ context.Context, groupID, role string) error {
	g, ok := f.groups[groupID]
	if !ok {
		return notFound()
	}
	out := g.Roles[:0]
	for _, r := range g.Roles {
		if r != role {
			out = append(out, r)
		}
	}
	g.Roles = out
	return nil
}

func (f *fakeIdentity) CountActiveSessions(context.Context) (int, error) { return 3, nil }

func (f *fakeIdentity) ListRoles(context.Context) ([]webkeycloak.IdentityRole, error) {
	out := []webkeycloak.IdentityRole{}
	for _, r := range f.roles {
		out = append(out, *r)
	}
	return out, nil
}

func (f *fakeIdentity) CreateRole(_ context.Context, name, description string) (*webkeycloak.IdentityRole, error) {
	if _, ok := f.roles[name]; ok {
		return nil, conflict()
	}
	r := &webkeycloak.IdentityRole{Name: name, Description: description}
	f.roles[name] = r
	return r, nil
}

func (f *fakeIdentity) UpdateRoleDescription(_ context.Context, name, description string) error {
	r, ok := f.roles[name]
	if !ok {
		return notFound()
	}
	r.Description = description
	return nil
}

func (f *fakeIdentity) DeleteRole(_ context.Context, name string) error {
	if _, ok := f.roles[name]; !ok {
		return notFound()
	}
	delete(f.roles, name)
	return nil
}

var _ IdentityAdmin = (*fakeIdentity)(nil)

func adminClaims(_ *http.Request) (*auth.Claims, bool) {
	return &auth.Claims{PreferredUsername: "alice", Groups: []string{"admin"}}, true
}

func userClaims(_ *http.Request) (*auth.Claims, bool) {
	return &auth.Claims{PreferredUsername: "bob", Groups: []string{"users"}}, true
}

func newIdentityHandler(t *testing.T, fake *fakeIdentity, claims func(*http.Request) (*auth.Claims, bool)) http.Handler {
	t.Helper()
	sc := cache.NewServiceCache()
	sc.Add(&app.App{
		UID: "svc-grafana", Name: "grafana", Namespace: "monitoring", Hostname: "grafana.example.com",
		LandingPage: &app.LandingPage{Enabled: true, DisplayName: "Grafana", Visibility: "private", RequiredGroups: []string{"admin"}},
	})
	sc.Add(&app.App{
		UID: "svc-docs", Name: "docs", Namespace: "docs", Hostname: "docs.example.com",
		LandingPage: &app.LandingPage{Enabled: true, DisplayName: "Docs", Visibility: "public"},
	})
	opts := []HandlerOption{WithClaimsExtractor(claims)}
	if fake != nil {
		opts = append(opts, WithIdentityAdmin(fake))
	}
	return NewHandler(sc, nil, true, nil, nil, opts...).Routes()
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func TestAdminIdentity_NonAdminGets403(t *testing.T) {
	h := newIdentityHandler(t, newFakeIdentity(), userClaims)
	for _, path := range []string{"/api/v1/admin/users", "/api/v1/admin/groups", "/api/v1/admin/roles", "/api/v1/admin/services"} {
		if rec := do(t, h, http.MethodGet, path, nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s: want 403, got %d", path, rec.Code)
		}
	}
}

func TestAdminIdentity_NoClientGets501_ButServicesStillWork(t *testing.T) {
	h := newIdentityHandler(t, nil, adminClaims)
	if rec := do(t, h, http.MethodGet, "/api/v1/admin/users", nil); rec.Code != http.StatusNotImplemented {
		t.Fatalf("want 501, got %d", rec.Code)
	}
	rec := do(t, h, http.MethodGet, "/api/v1/admin/services", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	services := decode[[]AdminService](t, rec)
	if len(services) != 2 {
		t.Fatalf("want 2 services, got %d", len(services))
	}
}

func TestAdminIdentity_ListUsersFilters(t *testing.T) {
	h := newIdentityHandler(t, newFakeIdentity(), adminClaims)

	all := decode[AdminUsersResponse](t, do(t, h, http.MethodGet, "/api/v1/admin/users", nil))
	if all.Total != 3 {
		t.Fatalf("want 3 users, got %d", all.Total)
	}
	byGroup := decode[AdminUsersResponse](t, do(t, h, http.MethodGet, "/api/v1/admin/users?group=g-admin", nil))
	if byGroup.Total != 1 || byGroup.Users[0].Username != "alice" {
		t.Fatalf("group filter: %+v", byGroup)
	}
	byQ := decode[AdminUsersResponse](t, do(t, h, http.MethodGet, "/api/v1/admin/users?q=BOB@", nil))
	if byQ.Total != 1 || byQ.Users[0].Username != "bob" {
		t.Fatalf("q filter: %+v", byQ)
	}
	disabled := decode[AdminUsersResponse](t, do(t, h, http.MethodGet, "/api/v1/admin/users?enabled=false", nil))
	if disabled.Total != 1 || disabled.Users[0].Username != "eve" {
		t.Fatalf("enabled filter: %+v", disabled)
	}
}

func TestAdminIdentity_UserMutations(t *testing.T) {
	fake := newFakeIdentity()
	h := newIdentityHandler(t, fake, adminClaims)

	rec := do(t, h, http.MethodPatch, "/api/v1/admin/users/u-alice", AdminUserPatch{Enabled: boolP(false)})
	if rec.Code != http.StatusOK || decode[webkeycloak.IdentityUser](t, rec).Enabled {
		t.Fatalf("patch enabled: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPut, "/api/v1/admin/users/u-bob/groups/g-admin", nil)
	if rec.Code != http.StatusOK || !contains(decode[webkeycloak.IdentityUser](t, rec).Groups, "g-admin") {
		t.Fatalf("add group: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodDelete, "/api/v1/admin/users/u-bob/groups/g-admin", nil)
	if rec.Code != http.StatusOK || contains(decode[webkeycloak.IdentityUser](t, rec).Groups, "g-admin") {
		t.Fatalf("remove group: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPut, "/api/v1/admin/users/u-eve/roles/viewer", nil)
	if rec.Code != http.StatusOK || !contains(decode[webkeycloak.IdentityUser](t, rec).Roles, "viewer") {
		t.Fatalf("assign role: %d %s", rec.Code, rec.Body.String())
	}

	if rec := do(t, h, http.MethodGet, "/api/v1/admin/users/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing user: want 404, got %d", rec.Code)
	}
	if rec := do(t, h, http.MethodPut, "/api/v1/admin/users/u-eve/groups/g-missing", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing group: want 404, got %d", rec.Code)
	}
}

func TestAdminIdentity_DeleteUser(t *testing.T) {
	fake := newFakeIdentity()
	self := func(_ *http.Request) (*auth.Claims, bool) {
		return &auth.Claims{
			RegisteredClaims:  jwt.RegisteredClaims{Subject: "u-alice"},
			PreferredUsername: "alice",
			Groups:            []string{"admin"},
		}, true
	}
	h := newIdentityHandler(t, fake, self)

	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/users/u-alice", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("self delete: want 403, got %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/users/u-bob", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := fake.users["u-bob"]; ok {
		t.Fatal("bob should be gone")
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/users/u-bob", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: want 404, got %d", rec.Code)
	}
}

func TestAdminIdentity_Bulk(t *testing.T) {
	fake := newFakeIdentity()
	h := newIdentityHandler(t, fake, adminClaims)

	rec := do(t, h, http.MethodPost, "/api/v1/admin/users/bulk", BulkUsersRequest{
		UserIDs: []string{"u-bob", "u-eve", "u-missing"}, Action: "addToGroup", GroupID: "g-admin",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk: %d %s", rec.Code, rec.Body.String())
	}
	if got := decode[BulkUsersResponse](t, rec).Updated; got != 2 {
		t.Fatalf("want 2 updated, got %d", got)
	}
	if !contains(fake.users["u-eve"].Groups, "g-admin") {
		t.Fatal("eve should be in g-admin")
	}

	rec = do(t, h, http.MethodPost, "/api/v1/admin/users/bulk", BulkUsersRequest{UserIDs: []string{"u-bob"}, Action: "assignRole"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing role: want 400, got %d", rec.Code)
	}
	rec = do(t, h, http.MethodPost, "/api/v1/admin/users/bulk", BulkUsersRequest{UserIDs: []string{"u-bob"}, Action: "explode"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown action: want 400, got %d", rec.Code)
	}
	rec = do(t, h, http.MethodPost, "/api/v1/admin/users/bulk", BulkUsersRequest{UserIDs: []string{"u-alice", "u-bob"}, Action: "disable"})
	if rec.Code != http.StatusOK || fake.users["u-alice"].Enabled || fake.users["u-bob"].Enabled {
		t.Fatalf("disable: %d", rec.Code)
	}
}

func TestAdminIdentity_Groups(t *testing.T) {
	fake := newFakeIdentity()
	h := newIdentityHandler(t, fake, adminClaims)

	rec := do(t, h, http.MethodPost, "/api/v1/admin/groups", AdminGroupInput{Name: "Data Science"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad name: want 400, got %d", rec.Code)
	}
	rec = do(t, h, http.MethodPost, "/api/v1/admin/groups", AdminGroupInput{Name: "data-science", Description: "DS team"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	created := decode[webkeycloak.IdentityGroup](t, rec)
	if created.Description != "DS team" || created.Path != "/data-science" {
		t.Fatalf("created: %+v", created)
	}
	if rec := do(t, h, http.MethodPost, "/api/v1/admin/groups", AdminGroupInput{Name: "data-science"}); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: want 409, got %d", rec.Code)
	}

	rec = do(t, h, http.MethodPatch, "/api/v1/admin/groups/"+created.ID, AdminGroupInput{Description: "renamed"})
	if rec.Code != http.StatusOK || decode[webkeycloak.IdentityGroup](t, rec).Description != "renamed" {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPut, "/api/v1/admin/groups/"+created.ID+"/members/u-bob", nil)
	if rec.Code != http.StatusOK || !contains(fake.users["u-bob"].Groups, created.ID) {
		t.Fatalf("add member: %d", rec.Code)
	}
	rec = do(t, h, http.MethodPut, "/api/v1/admin/groups/"+created.ID+"/roles/viewer", nil)
	if rec.Code != http.StatusOK || !contains(decode[webkeycloak.IdentityGroup](t, rec).Roles, "viewer") {
		t.Fatalf("add role: %d %s", rec.Code, rec.Body.String())
	}

	// admin is referenced by the grafana gate → 409 with the service listed.
	rec = do(t, h, http.MethodDelete, "/api/v1/admin/groups/g-admin", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("delete referenced: want 409, got %d", rec.Code)
	}
	if e := decode[AdminError](t, rec); len(e.Services) != 1 || e.Services[0] != "svc-grafana" {
		t.Fatalf("conflict body: %+v", e)
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/groups/"+created.ID, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/groups/"+created.ID, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: want 404, got %d", rec.Code)
	}
}

func TestAdminIdentity_Roles(t *testing.T) {
	fake := newFakeIdentity()
	h := newIdentityHandler(t, fake, adminClaims)

	rec := do(t, h, http.MethodGet, "/api/v1/admin/roles", nil)
	roles := decode[[]webkeycloak.IdentityRole](t, rec)
	if len(roles) != 4 {
		t.Fatalf("want 4 roles, got %d", len(roles))
	}

	rec = do(t, h, http.MethodPost, "/api/v1/admin/roles", AdminRoleInput{Name: "dashboard-editor", Description: "edits"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodPatch, "/api/v1/admin/roles/dashboard-editor", AdminRoleInput{Description: "edits dashboards"})
	if rec.Code != http.StatusOK || decode[webkeycloak.IdentityRole](t, rec).Description != "edits dashboards" {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodPatch, "/api/v1/admin/roles/offline_access", AdminRoleInput{Description: "x"}); rec.Code != http.StatusForbidden {
		t.Fatalf("patch built-in: want 403, got %d", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/roles/default-roles-nebari", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("delete built-in: want 403, got %d", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/roles/dashboard-editor", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", rec.Code)
	}
	if rec := do(t, h, http.MethodDelete, "/api/v1/admin/roles/dashboard-editor", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: want 404, got %d", rec.Code)
	}
}

func TestAdminOverview(t *testing.T) {
	h := newIdentityHandler(t, newFakeIdentity(), adminClaims)
	rec := do(t, h, http.MethodGet, "/api/v1/admin/overview", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}
	o := decode[AdminOverview](t, rec)
	if !o.IdentityAvailable || o.Users.Total != 3 || o.Users.Disabled != 1 || o.Users.WithoutGroups != 1 {
		t.Fatalf("users: %+v", o.Users)
	}
	if o.Groups != 2 || o.Roles != 2 {
		t.Fatalf("groups/roles: %d/%d", o.Groups, o.Roles)
	}
	if o.Services.Total != 2 || o.Services.Public != 1 || o.Services.Gated != 1 || o.Services.Unknown != 2 {
		t.Fatalf("services: %+v", o.Services)
	}
	if o.ActiveSessions == nil || *o.ActiveSessions != 3 {
		t.Fatalf("sessions: %v", o.ActiveSessions)
	}
	if o.AccessRequestsAvailable {
		t.Fatal("no access-request store was configured")
	}

	// Without an identity backend the endpoint still answers with service figures.
	h = newIdentityHandler(t, nil, adminClaims)
	o = decode[AdminOverview](t, do(t, h, http.MethodGet, "/api/v1/admin/overview", nil))
	if o.IdentityAvailable || o.Services.Total != 2 {
		t.Fatalf("no identity: %+v", o)
	}
	if rec := do(t, newIdentityHandler(t, nil, userClaims), http.MethodGet, "/api/v1/admin/overview", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: want 403, got %d", rec.Code)
	}
}

func TestAdminServiceHealthHistory(t *testing.T) {
	sc := cache.NewServiceCache()
	sc.Add(&app.App{
		UID: "svc-grafana", Name: "grafana", Namespace: "monitoring", Hostname: "grafana.example.com",
		LandingPage: &app.LandingPage{Enabled: true, DisplayName: "Grafana", Visibility: "public",
			HealthCheck: &app.HealthCheck{Enabled: true, Path: "/"}},
	})
	base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	for i, st := range []string{"healthy", "healthy", "unhealthy", "healthy", "healthy"} {
		at := base.Add(time.Duration(i) * time.Minute)
		sc.UpdateHealth("svc-grafana", &cache.HealthStatus{Status: st, LastCheck: &at})
	}
	h := NewHandler(sc, nil, true, nil, nil, WithClaimsExtractor(adminClaims)).Routes()

	rec := do(t, h, http.MethodGet, "/api/v1/admin/services/svc-grafana/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rec.Code, rec.Body.String())
	}
	got := decode[AdminServiceHealthHistory](t, rec)
	if got.Status != "healthy" || len(got.History) != 5 || got.Samples != 5 {
		t.Fatalf("history: %+v", got)
	}
	if got.UptimePercent == nil || *got.UptimePercent != 80 {
		t.Fatalf("uptime: %v", got.UptimePercent)
	}
	if got.StreakStatus != "healthy" || got.StreakSince == nil || !got.StreakSince.Equal(base.Add(3*time.Minute)) {
		t.Fatalf("streak: %s since %v", got.StreakStatus, got.StreakSince)
	}

	list := decode[[]AdminService](t, do(t, h, http.MethodGet, "/api/v1/admin/services", nil))
	if len(list) != 1 || list[0].Health == nil || list[0].Health.Samples != 5 {
		t.Fatalf("list health: %+v", list)
	}
	if rec := do(t, h, http.MethodGet, "/api/v1/admin/services/nope/health", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: want 404, got %d", rec.Code)
	}

	// History is capped and dropped with the service.
	for i := 0; i < cache.HealthHistoryLimit+10; i++ {
		sc.UpdateHealth("svc-grafana", &cache.HealthStatus{Status: "healthy"})
	}
	if n := len(sc.HealthHistory("svc-grafana")); n != cache.HealthHistoryLimit {
		t.Fatalf("cap: got %d", n)
	}
	sc.Remove("svc-grafana")
	if n := len(sc.HealthHistory("svc-grafana")); n != 0 {
		t.Fatalf("after remove: got %d", n)
	}
}

type fakePackLister struct {
	apps []packs.ArgoApp
	err  error
}

func (f fakePackLister) ListArgoApps(context.Context) ([]packs.ArgoApp, error) { return f.apps, f.err }

func TestAdminPacks(t *testing.T) {
	sc := cache.NewServiceCache()
	sc.Add(&app.App{
		UID: "svc-grafana", Name: "grafana", Namespace: "monitoring", Hostname: "grafana.example.com",
		Labels:      map[string]string{"helm.sh/chart": "nebari-lgtm-pack-0.2.0", "app.kubernetes.io/instance": "lgtm-pack"},
		Annotations: map[string]string{"argocd.argoproj.io/tracking-id": "lgtm-pack:reconcilers.nebari.dev/NebariApp:monitoring/grafana"},
		LandingPage: &app.LandingPage{Enabled: true, DisplayName: "Grafana", Visibility: "private", RequiredGroups: []string{"admin"}},
	})
	lister := fakePackLister{apps: []packs.ArgoApp{{Name: "lgtm-pack", Tier: packs.TierPack, Namespace: "monitoring", Chart: "nebari-lgtm-pack", TargetRevision: "0.2.0", SyncStatus: "Synced", HealthStatus: "Healthy"}}}
	h := NewHandler(sc, nil, true, nil, nil, WithClaimsExtractor(adminClaims), WithPackLister(lister)).Routes()

	res := decode[AdminPacksResponse](t, do(t, h, http.MethodGet, "/api/v1/admin/packs", nil))
	if !res.ArgoCDAvailable || len(res.Packs) != 1 || res.Packs[0].Argo == nil || len(res.Packs[0].Services) != 1 {
		t.Fatalf("packs: %+v", res)
	}
	one := decode[packs.Pack](t, do(t, h, http.MethodGet, "/api/v1/admin/packs/lgtm-pack", nil))
	if one.Name != "lgtm-pack" || one.Services[0].RequiredGroups[0] != "admin" {
		t.Fatalf("pack: %+v", one)
	}
	if rec := do(t, h, http.MethodGet, "/api/v1/admin/packs/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: want 404, got %d", rec.Code)
	}

	// ArgoCD unreadable: degraded but still answers from labels.
	h = NewHandler(sc, nil, true, nil, nil, WithClaimsExtractor(adminClaims), WithPackLister(fakePackLister{err: errNotFound})).Routes()
	res = decode[AdminPacksResponse](t, do(t, h, http.MethodGet, "/api/v1/admin/packs", nil))
	if res.ArgoCDAvailable || res.Error == "" || len(res.Packs) != 1 || res.Packs[0].Argo != nil || res.Packs[0].ChartVersion != "0.2.0" {
		t.Fatalf("degraded: %+v", res)
	}
	if rec := do(t, newIdentityHandler(t, nil, userClaims), http.MethodGet, "/api/v1/admin/packs", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin: want 403, got %d", rec.Code)
	}
}

func TestAdminIdentity_ServiceGate(t *testing.T) {
	h := newIdentityHandler(t, nil, adminClaims)
	rec := do(t, h, http.MethodGet, "/api/v1/admin/services/svc-grafana", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	s := decode[AdminService](t, rec)
	if s.Visibility != "private" || len(s.RequiredGroups) != 1 || s.RequiredGroups[0] != "admin" {
		t.Fatalf("gate: %+v", s)
	}
	if rec := do(t, h, http.MethodGet, "/api/v1/admin/services/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing: want 404, got %d", rec.Code)
	}
}

func boolP(b bool) *bool { return &b }
