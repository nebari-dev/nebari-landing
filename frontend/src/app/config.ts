// Runtime configuration loaded from /config.json at startup.
// config.json is rendered by the Helm chart (values.yaml → frontend.*) and
// mounted into the nginx container — no rebuild needed to change settings.
//
// Call loadAppConfig() once before the app renders (see main.tsx).
// All subsequent callers use getAppConfig() to access the cached value.

export type AppConfig = {
  keycloak: { url: string; realm: string; clientId: string };
  /** Optional page title override shown in the browser tab. */
  title?: string;
  /** Optional URL to a custom logo image rendered in the header. */
  logoUrl?: string;
};

let _config: AppConfig | null = null;

/**
 * Fetch and cache /config.json. Safe to call multiple times — the network
 * request only happens once.
 */
export async function loadAppConfig(): Promise<AppConfig> {
  if (_config) return _config;
  const res = await fetch("/config.json");
  if (!res.ok) throw new Error(`Failed to load /config.json: ${res.status}`);
  _config = (await res.json()) as AppConfig;
  return _config;
}

/** Returns the cached config, or null if loadAppConfig() has not yet resolved. */
export function getAppConfig(): AppConfig | null {
  return _config;
}
