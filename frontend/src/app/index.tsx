import {
  useState, useEffect
} from "react";

import
  Header
from "../components/header";

import
  Content
from "../components/content";

import { initKeycloak, signIn, keycloak } from "../auth/keycloak";

export default function App() {
  const [isDarkMode, setIsDarkMode] = useState(false);
  const [isAuthReady, setIsAuthReady] = useState(false);

  useEffect(() => {
    initKeycloak()
      .then(() => setIsAuthReady(true))
      .catch(err => { console.error(err); setIsAuthReady(true); });
  }, []);

  if (!isAuthReady) {
    return null;
  }

  return (
    <div className={isDarkMode ? "app-shell app-shell--dark" : "app-shell app-shell--light"}>
      <Header
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode((prev) => !prev)}
        onSignIn={signIn}
        signInLabel={keycloak.authenticated ? "Signed in" : "Sign in"}
      />
      <Content />
    </div>
  )
}
