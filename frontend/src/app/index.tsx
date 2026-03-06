import { useState } from "react";

import Header from "../components/header";
import Content from "../components/content";

import { useUser } from "../auth/user";

export default function App() {
  const [isDarkMode, setIsDarkMode] = useState(false);

  // oauth2-proxy enforces auth at the network level — the app only renders
  // for authenticated users, so there is no auth-init gate needed here.
  const { user } = useUser();

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
