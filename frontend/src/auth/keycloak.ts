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
  // Redirect to /public after clearing the session.
  // /public is whitelisted in oauth2-proxy (--skip-auth-route), so the user
  // lands on the public page instead of being bounced back to Keycloak login.
  window.location.href = "/oauth2/sign_out?rd=/public";
}
