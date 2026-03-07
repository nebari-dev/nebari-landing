import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import "./index.scss";
import "@trussworks/react-uswds/lib/index.css";

const root = createRoot(document.getElementById('root')!);

// Simple path-based routing without a router library.
// /public is the unauthenticated landing page — no Keycloak init needed.
// Every other route requires authentication: initKeycloak() performs the PKCE
// login flow and redirects to Keycloak if the user has no valid session.
if (window.location.pathname.startsWith('/public')) {
  const { default: PublicPage } = await import('../pages/PublicPage.tsx');
  root.render(
    <StrictMode>
      <PublicPage />
    </StrictMode>,
  );
} else {
  const { initKeycloak } = await import('../auth/keycloak.ts');
  await initKeycloak();

  const { default: App } = await import('./index.tsx');
  root.render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

