import {
  useState
} from "react";

import
  Header
from "../components/header";

import
  Content
from "../components/content";

function App() {
  const [isDarkMode, setIsDarkMode] = useState(false);

  return (
    <div className={isDarkMode ? "app-shell app-shell--dark" : "app-shell app-shell--light"}>
      <Header
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode((prev) => !prev)}
      />
      <Content />
    </div>
  )
}

export default App
