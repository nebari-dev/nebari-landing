import Keycloak from "keycloak-js";

export const keycloak = new Keycloak({
  url: import.meta.env.VITE_KEYCLOAK_URL,
  realm: import.meta.env.VITE_KEYCLOAK_REALM,
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID,
});

let keycloakInitPromise: Promise<boolean> | null = null;

export function initKeycloak() {
  if (!keycloakInitPromise) {
    keycloakInitPromise = keycloak.init({
      onLoad: "check-sso",
    });
  }
  return keycloakInitPromise;
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
