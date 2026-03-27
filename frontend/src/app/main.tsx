import {
  StrictMode
} from 'react'

import {
  createRoot
} from 'react-dom/client'

import {
  initKeycloak
} from '../auth/keycloak';

import { loadAppConfig, getAppConfig } from './config.ts';

import App from './index.tsx'

import "./index.css";

await loadAppConfig();
await initKeycloak();

const appConfig = getAppConfig();
if (appConfig?.title) {
  document.title = appConfig.title;
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
