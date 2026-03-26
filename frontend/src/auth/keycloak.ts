// Keycloak connection settings are served at runtime from /config.json,
// which is rendered by the Helm chart (values.yaml → frontend.keycloak.*)
// and mounted by the frontend ConfigMap into the nginx container.
// In local dev (Vite dev server / dev-watch) the file comes from
// frontend/public/config.json, which holds local defaults.
//
// Auth flow: the SPA uses keycloak-js with PKCE (S256) to authenticate
// directly with Keycloak and obtain an access token. The token is attached
// as `Authorization: Bearer <token>` on every API request by apiFetch().
// In production, nginx/oauth2-proxy may also inject the header server-side;
// both mechanisms are harmless when present together — whichever reaches
// the webapi first is validated the same way.

import Keycloak from "keycloak-js";
import { loadAppConfig } from "../app/config";

let _keycloak: Keycloak | null = null;

/**
 * Initialise Keycloak.js and perform the OIDC login flow (PKCE/S256).
 *
 * Must be called once before the authenticated app is rendered.
 * Redirects to the Keycloak login page if the user has no valid session;
 * the redirect back resumes this call transparently.
 *
 * Returns the initialised Keycloak instance.
 */
export async function initKeycloak(): Promise<Keycloak> {
  if (_keycloak) return _keycloak;

  const appConfig = await loadAppConfig();
  const cfg = appConfig.keycloak;
  const kc = new Keycloak({ url: cfg.url, realm: cfg.realm, clientId: cfg.clientId });

  await kc.init({
    onLoad: "login-required",
    pkceMethod: "S256",
    checkLoginIframe: false,
  });

  _keycloak = kc;
  return kc;
}

/** Returns the raw Keycloak instance (null until initKeycloak resolves). */
export function getKeycloakInstance(): Keycloak | null {
  return _keycloak;
}

/**
 * Returns the current access token, refreshing it first if it expires within
 * 30 seconds. Returns undefined if Keycloak has not been initialised or the
 * session is no longer valid (in which case keycloak-js triggers a re-login).
 */
export async function getToken(): Promise<string | undefined> {
  if (!_keycloak?.authenticated) return undefined;
  try {
    await _keycloak.updateToken(30);
  } catch {
    // Refresh failed (e.g. session expired) — force a new login.
    _keycloak.login();
    return undefined;
  }
  return _keycloak.token;
}

/** Trigger Keycloak login (redirects the browser). */
export function signIn() {
  if (_keycloak) {
    _keycloak.login({ redirectUri: window.location.origin + "/" });
  } else {
    // Keycloak not yet initialised (e.g. called from the public page).
    // Redirect to the authenticated zone which will trigger the login flow.
    window.location.href = "/";
  }
}

/** Trigger Keycloak logout (redirects the browser). */
export function signOut() {
  if (_keycloak) {
    _keycloak.logout({ redirectUri: window.location.origin + "/" });
  } else {
    window.location.href = "/";
  }
}
