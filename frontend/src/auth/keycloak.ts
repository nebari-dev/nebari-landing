import Keycloak from "keycloak-js";

type KeycloakConfig = { url: string; realm: string; clientId: string };

declare global {
  interface Window {
    __PW_E2E_AUTH__?: {
      authenticated: boolean;
      token?: string;
      idTokenParsed?: Record<string, string>;
    };
  }
}

let _config: KeycloakConfig | null = null;
let _keycloak: Keycloak | null = null;

async function loadKeycloakConfig(): Promise<KeycloakConfig> {
  if (_config) return _config;
  const res = await fetch("/config.json");
  if (!res.ok) throw new Error(`Failed to load /config.json: ${res.status}`);
  const data = await res.json();
  _config = data.keycloak as KeycloakConfig;
  return _config;
}

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

  const cfg = await loadKeycloakConfig();
  const kc = new Keycloak({
    url: cfg.url,
    realm: cfg.realm,
    clientId: cfg.clientId,
  });

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

export async function getToken(): Promise<string | undefined> {
  if (!_keycloak?.authenticated) return undefined;

  try {
    await _keycloak.updateToken(30);
  } catch {
    _keycloak.login();
    return undefined;
  }

  return _keycloak.token;
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
