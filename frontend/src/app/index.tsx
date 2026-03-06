import { useState, useEffect } from "react";

import Header from "../components/header";
import Content from "../components/content";

import { initKeycloak } from "../auth/keycloak";
import { useUser } from "../auth/user";

export default function App() {
  const [isDarkMode, setIsDarkMode] = useState(false);
  const [isAuthReady, setIsAuthReady] = useState(false);

  useEffect(() => {
    initKeycloak()
      .then(() => setIsAuthReady(true))
      .catch(err => { console.error(err); setIsAuthReady(true); });
  }, []);

  // Fetch user from oauth2-proxy — null when not logged in.
  const { user } = useUser();

  if (!isAuthReady) {
    return null;
  }

  return (
    <div className={isDarkMode ? "app-shell app-shell--dark" : "app-shell app-shell--light"}>
      <Header
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode((prev) => !prev)}
        user={user}
      />
      <Content />
    </div>
  );
}
