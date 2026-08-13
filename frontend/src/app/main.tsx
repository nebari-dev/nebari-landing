import { StrictMode } from "react";

import { createRoot } from "react-dom/client";

import { initKeycloak } from "../auth/keycloak";
import { ThemeProvider } from "../hooks/theme-provider";
import { isThemeMode } from "../hooks/use-theme-preference";
import { applyAppConfig, getAppConfig, loadAppConfig } from "./config.ts";
import App from "./index.tsx";

import "./index.css";

const THEME_MODE_STORAGE_KEY = "launchpad:themeMode";

// One-time migration of the legacy boolean dark-mode preference to the
// `light | dark | system` key read by `@nebari/use-theme-preference` (and by
// the bootstrap script in index.html). Runs before render so the first React
// pass already sees the migrated value.
try {
  const stored = window.localStorage.getItem(THEME_MODE_STORAGE_KEY);
  if (stored === null || !isThemeMode(stored)) {
    const legacy = window.localStorage.getItem("launchpad:isDarkMode");
    if (legacy === "true") window.localStorage.setItem(THEME_MODE_STORAGE_KEY, "dark");
    if (legacy === "false") window.localStorage.setItem(THEME_MODE_STORAGE_KEY, "light");
  }
} catch {
  // Storage unavailable (private browsing, disabled) — skip migration.
}

// Clear stale oauth2-proxy session cookies left by the previous architecture.
// Users who visit after the oauth2-proxy sidecar is removed would otherwise
// carry a dead session cookie that confuses some OIDC flows.
for (const cookie of document.cookie.split(";")) {
  const name = cookie.trim().split("=")[0];
  if (name.startsWith("_oauth2_proxy")) {
    // biome-ignore lint/suspicious/noDocumentCookie: Intentionally expire legacy, script-accessible oauth2-proxy cookies with Max-Age=0.
    document.cookie = `${name}=; Max-Age=0; path=/`;
  }
}

// Optional MSW layer for local frontend dev: start the service worker before
// any Keycloak / config fetches so every subsequent network call (REST + WS)
// is intercepted. Playwright supplies its own route mocks and blocks service
// workers, so skip MSW when the test-only authentication state is present.
// Production builds never enable this branch. See docs/dev-quickstart.md.
if (import.meta.env.VITE_USE_MOCKS === "1" && !window.__PW_E2E_AUTH__) {
  const { worker } = await import("../mocks/browser");
  await worker.start({
    onUnhandledRequest: "bypass",
    serviceWorker: { url: "/mockServiceWorker.js" },
  });
}

await loadAppConfig();
await initKeycloak();

const appConfig = getAppConfig();
if (appConfig) applyAppConfig(appConfig);

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Root element not found");
}

createRoot(rootElement).render(
  <StrictMode>
    <ThemeProvider storageKey={THEME_MODE_STORAGE_KEY}>
      <App />
    </ThemeProvider>
  </StrictMode>,
);
