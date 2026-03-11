// import {
//   StrictMode
// } from 'react'

import {
  createRoot
} from 'react-dom/client'

import {
  initKeycloak
} from '../auth/keycloak.ts';

import App from './index.tsx'
import "./index.scss";
import "@trussworks/react-uswds/lib/index.css";

await initKeycloak();

createRoot(document.getElementById('root')!).render(
  // <StrictMode>
  <App />
  //</StrictMode>
)
