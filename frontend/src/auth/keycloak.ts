import Keycloak from "keycloak-js";
import { loadAppConfig } from "../app/config";

declare global {
  interface Window {
    __PW_E2E_AUTH__?: {
      authenticated: boolean;
      token?: string;
      idTokenParsed?: Record<string, string>;
    };
  }
}

let _keycloak: Keycloak | null = null;

export async function initKeycloak(): Promise<Keycloak> {
  if (_keycloak) return _keycloak;

  const injected = window.__PW_E2E_AUTH__;
  if (import.meta.env.MODE !== "production" && injected?.authenticated) {
    _keycloak = {
      authenticated: true,
      token: injected.token,
      idTokenParsed: injected.idTokenParsed,
      updateToken: async () => true,
      login: async () => {},
      logout: async () => {},
    } as unknown as Keycloak;

    return _keycloak;
  }

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

export function getKeycloakInstance(): Keycloak | null {
  return _keycloak;
}

/**
 * Returns a valid access token, refreshing if needed.
 *
 * Throws SessionExpiredError when the refresh token is expired and a
 * full re-authentication is required. Callers should catch this and
 * avoid making API calls (the redirect to Keycloak is already in flight).
 */
export class SessionExpiredError extends Error {
  constructor() {
    super("Session expired — redirecting to login");
    this.name = "SessionExpiredError";
  }
}

export async function getToken(): Promise<string> {
  if (!_keycloak?.authenticated) {
    throw new SessionExpiredError();
  }

  try {
    await _keycloak.updateToken(30);
  } catch {
    _keycloak.login();
    throw new SessionExpiredError();
  }

  return _keycloak.token!;
}

export function signIn() {
  if (_keycloak) {
    _keycloak.login({ redirectUri: window.location.origin + "/" });
  } else {
    window.location.href = "/";
  }
}

export function signOut() {
  if (_keycloak) {
    _keycloak.logout({ redirectUri: window.location.origin + "/" });
  } else {
    window.location.href = "/";
  }
}
