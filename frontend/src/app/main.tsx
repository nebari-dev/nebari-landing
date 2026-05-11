import {
  StrictMode
} from 'react'

import {
  createRoot
} from 'react-dom/client'

import {
  initKeycloak
} from '../auth/keycloak';

import { loadAppConfig, getAppConfig, applyAppConfig } from './config.ts';

import App from './index.tsx'

import "./index.css";

// Clear stale oauth2-proxy session cookies left by the previous architecture.
// Users who visit after the oauth2-proxy sidecar is removed would otherwise
// carry a dead session cookie that confuses some OIDC flows.
for (const cookie of document.cookie.split(";")) {
  const name = cookie.trim().split("=")[0];
  if (name.startsWith("_oauth2_proxy")) {
    document.cookie = `${name}=; Max-Age=0; path=/`;
  }
}

await loadAppConfig();
await initKeycloak();

const appConfig = getAppConfig();
if (appConfig) applyAppConfig(appConfig);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
