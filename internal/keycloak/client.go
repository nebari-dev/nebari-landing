// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

// Package keycloak provides a lightweight Keycloak Admin REST API client for
// the webapi service.
//
// It intentionally mirrors the environment-variable interface used by the
// operator (internal/config/auth.go) so that the same Deployment env block
// works for both binaries:
//
//	KEYCLOAK_URL            – Keycloak base URL (Keycloak 17+; no /auth context root)
//	KEYCLOAK_REALM          – target realm  (default: nebari)
//	KEYCLOAK_ADMIN_USERNAME – master-realm admin username
//	KEYCLOAK_ADMIN_PASSWORD – master-realm admin password
//
// For Phase 2 only master-realm admin credentials are required; the client
// logs into the master realm, then operates on KEYCLOAK_REALM.
package keycloak

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Nerzal/gocloak/v13"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var log = ctrl.Log.WithName("keycloak-admin")

// Config holds the credentials and target needed by Client.
type Config struct {
	// URL is the Keycloak base URL, e.g. http://keycloak:8080
	URL string
	// Realm is the target realm in which users and groups are managed.
	Realm string
	// AdminUsername is the master-realm administrator username.
	AdminUsername string
	// AdminPassword is the master-realm administrator password.
	AdminPassword string
}

// ConfigFromEnv builds a Config from environment variables using the same
// variable names as the operator (internal/config/auth.go).
//
// Required variables: KEYCLOAK_ADMIN_USERNAME, KEYCLOAK_ADMIN_PASSWORD
// Optional:
//
//	KEYCLOAK_URL   (default: http://keycloak-keycloakx-http.keycloak.svc.cluster.local:8080)
//	KEYCLOAK_REALM (default: nebari)
func ConfigFromEnv() (Config, error) {
	url := os.Getenv("KEYCLOAK_URL")
	if url == "" {
		url = "http://keycloak-keycloakx-http.keycloak.svc.cluster.local:8080"
	}
	realm := os.Getenv("KEYCLOAK_REALM")
	if realm == "" {
		realm = "nebari"
	}
	user := os.Getenv("KEYCLOAK_ADMIN_USERNAME")
	pass := os.Getenv("KEYCLOAK_ADMIN_PASSWORD")
	if user == "" || pass == "" {
		return Config{}, errors.New(
			"KEYCLOAK_ADMIN_USERNAME and KEYCLOAK_ADMIN_PASSWORD must be set to enable Keycloak admin operations",
		)
	}
	return Config{URL: url, Realm: realm, AdminUsername: user, AdminPassword: pass}, nil
}

// Client is a thin wrapper around gocloak for admin operations needed by the webapi.
// It re-authenticates on every call (tokens are short-lived and calls are infrequent).
type Client struct {
	cfg Config
}

// New returns a new Client for the given config.
func New(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// NewFromEnv is a convenience constructor that calls ConfigFromEnv and New.
// Returns nil + error when env vars are missing.
func NewFromEnv() (*Client, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return New(cfg), nil
}

// NewFromEnvWithK8sClient is like NewFromEnv but also supports cross-namespace
// secret lookup via KEYCLOAK_ADMIN_SECRET_NAME / KEYCLOAK_ADMIN_SECRET_NAMESPACE,
// mirroring the operator's LoadKeycloakCredentials pattern.
//
// Precedence for credentials:
//  1. Kubernetes Secret (KEYCLOAK_ADMIN_SECRET_NAME + KEYCLOAK_ADMIN_SECRET_NAMESPACE)
//  2. Direct env vars   (KEYCLOAK_ADMIN_USERNAME  + KEYCLOAK_ADMIN_PASSWORD)
//
// Secret keys accepted (in priority order): "username" or "admin-username",
// "password" or "admin-password" — identical to the operator's convention.
// Returns nil + error only when no credentials could be resolved.
func NewFromEnvWithK8sClient(ctx context.Context, k8sClient client.Client) (*Client, error) {
	url := os.Getenv("KEYCLOAK_URL")
	if url == "" {
		url = "http://keycloak-keycloakx-http.keycloak.svc.cluster.local:8080"
	}
	realm := os.Getenv("KEYCLOAK_REALM")
	if realm == "" {
		realm = "nebari"
	}

	user := os.Getenv("KEYCLOAK_ADMIN_USERNAME")
	pass := os.Getenv("KEYCLOAK_ADMIN_PASSWORD")

	// Try the Kubernetes Secret first (supports cross-namespace access because
	// the webapi service account holds a RoleBinding in the keycloak namespace).
	secretName := os.Getenv("KEYCLOAK_ADMIN_SECRET_NAME")
	secretNS := os.Getenv("KEYCLOAK_ADMIN_SECRET_NAMESPACE")
	if secretNS == "" {
		secretNS = "keycloak"
	}

	if secretName != "" && k8sClient != nil {
		secret := &corev1.Secret{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: secretNS}, secret); err == nil {
			if u, ok := secret.Data["username"]; ok {
				user = string(u)
			} else if u, ok := secret.Data["admin-username"]; ok {
				user = string(u)
			} else if user != "" {
				// Secret has no username key; keep the value from KEYCLOAK_ADMIN_USERNAME.
				log.Info("Keycloak admin secret has no 'username'/'admin-username' key — using KEYCLOAK_ADMIN_USERNAME",
					"secret", secretName, "namespace", secretNS)
			} else {
				log.Info("Keycloak admin secret has no username key and KEYCLOAK_ADMIN_USERNAME is unset; add a 'username' key to the secret",
					"secret", secretName, "namespace", secretNS)
			}
			if p, ok := secret.Data["password"]; ok {
				pass = string(p)
			} else if p, ok := secret.Data["admin-password"]; ok {
				pass = string(p)
			}
			if user != "" && pass != "" {
				log.Info("Loaded Keycloak admin credentials from Kubernetes secret",
					"secret", secretName, "namespace", secretNS)
			}
		} else {
			log.Error(err, "Failed to read Keycloak admin secret; falling back to env vars",
				"secret", secretName, "namespace", secretNS)
		}
	}

	if user == "" || pass == "" {
		return nil, errors.New(
			"keycloak admin credentials not resolved: set KEYCLOAK_ADMIN_SECRET_NAME " +
				"(pointing to a secret with 'username'/'password' keys) " +
				"or KEYCLOAK_ADMIN_USERNAME + KEYCLOAK_ADMIN_PASSWORD",
		)
	}
	return New(Config{URL: url, Realm: realm, AdminUsername: user, AdminPassword: pass}), nil
}

// authenticate opens a gocloak client and obtains a short-lived admin token.
func (c *Client) authenticate(ctx context.Context) (*gocloak.GoCloak, *gocloak.JWT, error) {
	kc := gocloak.NewClient(c.cfg.URL)
	token, err := kc.LoginAdmin(ctx, c.cfg.AdminUsername, c.cfg.AdminPassword, "master")
	if err != nil {
		return nil, nil, fmt.Errorf("keycloak admin login failed: %w", err)
	}
	return kc, token, nil
}

// AddUserToGroup finds the user by username and the group by name in cfg.Realm,
// then adds the user to the group. The group is created if it does not exist.
// This operation is idempotent: adding a user to a group they already belong to
// is a no-op (Keycloak Admin API returns 204 regardless).
func (c *Client) AddUserToGroup(ctx context.Context, username, groupName string) error {
	kc, token, err := c.authenticate(ctx)
	if err != nil {
		return err
	}
	realm := c.cfg.Realm

	// ── locate user ──────────────────────────────────────────────────────────
	users, err := kc.GetUsers(ctx, token.AccessToken, realm, gocloak.GetUsersParams{
		Username: gocloak.StringP(username),
		Exact:    gocloak.BoolP(true),
	})
	if err != nil {
		return fmt.Errorf("looking up user %q in realm %q: %w", username, realm, err)
	}
	if len(users) == 0 {
		return fmt.Errorf("user %q not found in realm %q", username, realm)
	}
	userID := gocloak.PString(users[0].ID)

	// ── locate or create group ────────────────────────────────────────────────
	groups, err := kc.GetGroups(ctx, token.AccessToken, realm, gocloak.GetGroupsParams{
		Search: gocloak.StringP(groupName),
	})
	if err != nil {
		return fmt.Errorf("looking up group %q in realm %q: %w", groupName, realm, err)
	}

	var groupID string
	for _, g := range groups {
		if strings.EqualFold(gocloak.PString(g.Name), groupName) {
			groupID = gocloak.PString(g.ID)
			break
		}
	}

	if groupID == "" {
		// Group does not exist — create it.
		id, err := kc.CreateGroup(ctx, token.AccessToken, realm, gocloak.Group{
			Name: gocloak.StringP(groupName),
		})
		if err != nil {
			return fmt.Errorf("creating group %q in realm %q: %w", groupName, realm, err)
		}
		groupID = id
		log.Info("Created Keycloak group", "realm", realm, "group", groupName, "id", groupID)
	}

	// ── add user to group ─────────────────────────────────────────────────────
	if err := kc.AddUserToGroup(ctx, token.AccessToken, realm, userID, groupID); err != nil {
		return fmt.Errorf("adding user %q to group %q in realm %q: %w", username, groupName, realm, err)
	}

	log.Info("Added user to Keycloak group", "realm", realm, "user", username, "group", groupName)
	return nil
}
