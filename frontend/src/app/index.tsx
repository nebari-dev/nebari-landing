import { signOut } from "@/auth/keycloak";
import { useUser } from "@/auth/user";

import { Banner } from "../components/Banner";
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
      <Banner config={config?.banners?.top} />
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
      <Banner config={config?.banners?.bottom} />
    </main>
  );
}
