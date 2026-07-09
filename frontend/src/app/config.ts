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

export type BannerConfig = {
  /** Banner text, rendered as plain text (never HTML). */
  text: string;
  /** Optional CSS background color. Falls back to the theme foreground color. */
  background?: string;
  /** Optional CSS text color. Falls back to the theme background color. */
  foreground?: string;
};

export type AppConfig = {
  keycloak: { url: string; realm: string; clientId: string };
  /** Optional page title override shown in the browser tab. */
  title?: string;
  /** Optional URL to a custom logo image rendered in the header. */
  logoUrl?: string;
  /**
   * Optional URL to a custom dark-mode logo image rendered in the header.
   * Falls back to logoUrl (then the built-in Nebari logo) when empty.
   */
  logoUrlDark?: string;
  /** Optional URL to a custom favicon. */
  faviconUrl?: string;
  /** Optional CSS variable overrides for light and dark mode. */
  theme?: { light?: ThemeTokens; dark?: ThemeTokens };
  /**
   * Optional classification banners (e.g. CUI) pinned above the header and
   * below the page content.
   */
  banners?: { top?: BannerConfig; bottom?: BannerConfig };
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
  // Drop malformed logo URLs so a bad config value can't land in an <img src>
  // (defence-in-depth, mirroring the theme-token sanitisation below).
  _config.logoUrl = sanitizeUrl(_config.logoUrl);
  _config.logoUrlDark = sanitizeUrl(_config.logoUrlDark);
  return _config;
}

// Accept only non-empty, well-formed http(s) URLs or root-relative paths;
// anything else (including "") becomes undefined.
function sanitizeUrl(value: string | undefined): string | undefined {
  if (!value) return undefined;
  if (value.startsWith("/")) return value;
  try {
    const { protocol } = new URL(value);
    return protocol === "http:" || protocol === "https:" ? value : undefined;
  } catch {
    return undefined;
  }
}

/** Returns the cached config, or null if loadAppConfig() has not yet resolved. */
export function getAppConfig(): AppConfig | null {
  return _config;
}

// Block CSS injection vectors: rule terminators, braces, HTML chars, url()/expression()/javascript:
const UNSAFE_CSS = /[;<>{}"'\\]|url\s*\(|expression\s*\(|javascript:/i;

/** Returns the value unchanged if it is a safe CSS token, otherwise undefined. */
export function safeCssValue(value: string | undefined): string | undefined {
  return value && !UNSAFE_CSS.test(value) ? value : undefined;
}
const toKebab = (s: string) => s.replace(/([A-Z])/g, "-$1").toLowerCase();
const toCssVars = (tokens: Record<string, string>) =>
  Object.entries(tokens)
    .filter(([, v]) => v && !UNSAFE_CSS.test(v))
    .map(([k, v]) => `  --${toKebab(k)}: ${v};`)
    .join("\n");

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
      Object.assign(document.createElement("link"), { rel: "icon" })) as HTMLLinkElement;
    link.href = config.faviconUrl;
    document.head.appendChild(link);
  }

  if (config.theme) {
    let css = "";
    if (config.theme.light) {
      const vars = toCssVars(config.theme.light as Record<string, string>);
      if (vars) css += `:root {\n${vars}\n}\n`;
    }
    if (config.theme.dark) {
      const vars = toCssVars(config.theme.dark as Record<string, string>);
      if (vars) css += `.dark {\n${vars}\n}\n`;
    }
    if (css) {
      const style = document.createElement("style");
      style.textContent = css;
      document.head.appendChild(style);
    }
  }
}
