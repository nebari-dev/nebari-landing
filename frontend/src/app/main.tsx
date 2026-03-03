import {
  StrictMode
} from 'react'

import {
  createRoot
} from 'react-dom/client'

import App from './index.tsx'
import "./index.scss";
import "@trussworks/react-uswds/lib/index.css";

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
