// Runtime configuration loaded from /config.json at startup.
// config.json is rendered by the Helm chart (values.yaml → frontend.*) and
// mounted into the nginx container — no rebuild needed to change settings.
//
// Call loadAppConfig() once before the app renders (see main.tsx).
// All subsequent callers use getAppConfig() to access the cached value.

export type ThemeTokens = {
  primary?: string;
  primaryForeground?: string;
  background?: string;
  foreground?: string;
  secondary?: string;
  secondaryForeground?: string;
  muted?: string;
  mutedForeground?: string;
  accent?: string;
  accentForeground?: string;
  border?: string;
  ring?: string;
  radius?: string;
};

export type AppConfig = {
  keycloak: { url: string; realm: string; clientId: string };
  /** Optional page title override shown in the browser tab. */
  title?: string;
  /** Optional URL to a custom logo image rendered in the header. */
  logoUrl?: string;
  /** Optional URL to a custom favicon. */
  faviconUrl?: string;
  /** Optional CSS variable overrides for light and dark mode. */
  theme?: { light?: ThemeTokens; dark?: ThemeTokens };
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

// Block CSS injection vectors: rule terminators, braces, HTML chars, url()/expression()/javascript:
const UNSAFE_CSS = /[;<>{}"'\\]|url\s*\(|expression\s*\(|javascript:/i;
const toKebab = (s: string) => s.replace(/([A-Z])/g, '-$1').toLowerCase();
const toCssVars = (tokens: Record<string, string>) =>
  Object.entries(tokens)
    .filter(([, v]) => v && !UNSAFE_CSS.test(v))
    .map(([k, v]) => `  --${toKebab(k)}: ${v};`)
    .join('\n');

/**
 * Apply the loaded config to the document (title, favicon, theme CSS vars).
 * Should be called once after loadAppConfig() resolves.
 */
export function applyAppConfig(config: AppConfig): void {
  if (config.title) {
    document.title = config.title;
  }

  if (config.faviconUrl) {
    const link = (document.querySelector("link[rel~='icon']") ??
      Object.assign(document.createElement('link'), { rel: 'icon' })) as HTMLLinkElement;
    link.href = config.faviconUrl;
    document.head.appendChild(link);
  }

  if (config.theme) {
    let css = '';
    if (config.theme.light) {
      const vars = toCssVars(config.theme.light as Record<string, string>);
      if (vars) css += `:root {\n${vars}\n}\n`;
    }
    if (config.theme.dark) {
      const vars = toCssVars(config.theme.dark as Record<string, string>);
      if (vars) css += `.dark {\n${vars}\n}\n`;
    }
    if (css) {
      const style = document.createElement('style');
      style.textContent = css;
      document.head.appendChild(style);
    }
  }
}
