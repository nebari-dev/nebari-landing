import { StrictMode } from "react";

import { createRoot } from "react-dom/client";

import { initKeycloak } from "../auth/keycloak";
import { ThemeProvider } from "../hooks/ThemeContext";
import { applyAppConfig, getAppConfig, loadAppConfig } from "./config.ts";
import App from "./index.tsx";

import "./index.css";

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
// is intercepted. The worker is gated behind VITE_USE_MOCKS=1 so production
// builds never even bundle it. See docs/dev-quickstart.md.
if (import.meta.env.VITE_USE_MOCKS === "1") {
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
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </StrictMode>,
);
