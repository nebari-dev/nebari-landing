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
  // clears oauth2-proxy session cookie
  window.location.href = "/oauth2/sign_out?rd=/";
}
