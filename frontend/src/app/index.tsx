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
  const { services, onTogglePin } = useLaunchpadData(user);

  const config = getAppConfig();

  return (
    <main className="w-full pt-(--top-banner-height,0px) pb-(--bottom-banner-height,0px)">
      <Banner position="top" config={config?.banners?.top} />
      <Header
        isDarkMode={isDarkMode}
        themeMode={themeMode}
        onThemeChange={setThemeMode}
        user={user}
        onSignOut={() => signOut()}
        logoSrc={config?.logoUrl || undefined}
        logoSrcDark={config?.logoUrlDark || undefined}
      />

      <Content services={services} onTogglePin={onTogglePin} />
      <Banner position="bottom" config={config?.banners?.bottom} />
    </main>
  );
}
