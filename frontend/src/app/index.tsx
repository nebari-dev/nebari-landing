import { signOut } from "@/auth/keycloak";
import { useUser } from "@/auth/user";

import { Content } from "../components/Content";
import { Header } from "../components/Header";
import { useTheme } from "../hooks/ThemeContext";
import { useLaunchpadData } from "../hooks/useLaunchpadData";
import { getAppConfig } from "./config";

export default function App() {
  const { themeMode, isDarkMode, setThemeMode } = useTheme();
  const { user } = useUser();
  const { services, notifications, onNotificationsViewed, onTogglePin } = useLaunchpadData(user);

  const config = getAppConfig();

  return (
    <main className="w-full">
      <Header
        isDarkMode={isDarkMode}
        themeMode={themeMode}
        onThemeChange={setThemeMode}
        user={user}
        onSignOut={() => signOut()}
        notifications={notifications}
        onNotificationsViewed={onNotificationsViewed}
        logoSrc={config?.logoUrl || undefined}
        logoSrcDark={config?.logoUrlDark || undefined}
      />

      <Content services={services} onTogglePin={onTogglePin} />
    </main>
  );
}
