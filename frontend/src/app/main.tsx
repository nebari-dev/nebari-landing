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

await loadAppConfig();
await initKeycloak();

const appConfig = getAppConfig();
if (appConfig) applyAppConfig(appConfig);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
