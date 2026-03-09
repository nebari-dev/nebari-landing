// Keycloak connection settings are served at runtime from /config.json,
// which is rendered by the Helm chart (values.yaml → frontend.keycloak.*)
// and mounted by the frontend ConfigMap into the nginx container.
// In local dev (Vite dev server / dev-watch) the file comes from
// frontend/public/config.json, which holds local defaults.

type KeycloakConfig = { url: string; realm: string; clientId: string };

let _config: KeycloakConfig | null = null;

/**
 * Load Keycloak connection settings from the runtime config endpoint.
 * Cached after the first successful fetch — safe to call multiple times.
 */
export async function loadKeycloakConfig(): Promise<KeycloakConfig> {
  if (_config) return _config;
  const res = await fetch("/config.json");
  if (!res.ok) throw new Error(`Failed to load /config.json: ${res.status}`);
  const data = await res.json();
  _config = data.keycloak as KeycloakConfig;
  return _config;
}

export function signIn() {
  // rd = where to send the browser after auth finishes
  window.location.href = "/oauth2/start?rd=/";
}

export function signOut() {
  // /oauth2/sign_out clears the oauth2-proxy session cookie then — because
  // oauth2-proxy is configured with --oidc-logout-url pointing to Keycloak's
  // end-session endpoint — redirects to Keycloak to terminate the SSO session.
  // Keycloak bounces back to the app root via post_logout_redirect_uri.
  // Without this two-step flow, Keycloak would silently re-authenticate the
  // user on the very next page load (SSO session still active).
  window.location.href = "/oauth2/sign_out";
}
