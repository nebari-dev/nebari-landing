import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import "./index.scss";
import "@trussworks/react-uswds/lib/index.css";

const root = createRoot(document.getElementById('root')!);

// Simple path-based routing without a router library.
// oauth2-proxy enforces auth at the network level — /public is whitelisted
// via --skip-auth-route so PublicPage renders without a session cookie.
if (window.location.pathname.startsWith('/public')) {
  const { default: PublicPage } = await import('../pages/PublicPage.tsx');
  root.render(
    <StrictMode>
      <PublicPage />
    </StrictMode>,
  );
} else {
  const { default: App } = await import('./index.tsx');
  root.render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

