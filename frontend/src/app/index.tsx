import { signOut } from "@/auth/keycloak";
import { useUser } from "@/auth/user";

import { Banner } from "../components/Banner";
import { Content } from "../components/Content";
import { Header } from "../components/Header";
import { useTheme } from "../hooks/theme-provider";
import { useLaunchpadData } from "../hooks/useLaunchpadData";
import { getAppConfig } from "./config";

export default function App() {
  const { themeMode, isDarkMode, setThemeMode } = useTheme();
  const { user } = useUser();
  const {
    services,
    // Removed for now by the request in the meeting
    // notifications,
    // onNotificationsViewed,
    onTogglePin,
  } = useLaunchpadData(user);

  const config = getAppConfig();

  return (
    <main className="w-full pt-(--top-banner-height,0px) pb-(--bottom-banner-height,0px)">
      <Banner position="top" config={config?.banners?.top} />
      {/* Removed for now by the request in the meeting
      notifications={notifications}
      onNotificationsViewed={onNotificationsViewed}
      */}
      <Header
        isDarkMode={isDarkMode}
        themeMode={themeMode}
        onThemeChange={setThemeMode}
        user={user}
        onSignOut={() => signOut()}
        services={services}
        logoSrc={config?.logoUrl || undefined}
        logoSrcDark={config?.logoUrlDark || undefined}
        environment={config?.environment || undefined}
      />

      <Content services={services} onTogglePin={onTogglePin} />
      <Banner position="bottom" config={config?.banners?.bottom} />
    </main>
  );
}
