// Keycloak connection settings — baked in at build time via Vite.
// Defined as ARG/ENV in dev/Dockerfile; see VITE_KEYCLOAK_* in dev/Makefile
// for local defaults.
export const KEYCLOAK_URL       = import.meta.env.VITE_KEYCLOAK_URL       ?? "";
export const KEYCLOAK_REALM     = import.meta.env.VITE_KEYCLOAK_REALM     ?? "";
export const KEYCLOAK_CLIENT_ID = import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? "";

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
