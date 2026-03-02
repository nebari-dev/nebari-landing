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
  return keycloak.login();
}

export function signOut() {
  return keycloak.logout();
}
