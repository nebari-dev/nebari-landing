import { lazy, Suspense } from "react";
import { Route, Routes } from "react-router";
import { signIn, signOut } from "@/auth/keycloak";
import { useUser } from "@/auth/user";
import { Toaster } from "@/components/ui/toast";
import { useCallerIdentity } from "@/hooks/useCallerIdentity";

import { Banner } from "../components/Banner";
import { Content } from "../components/Content";
import { Header } from "../components/Header";
import { useTheme } from "../hooks/theme-provider";
import { useLaunchpadData } from "../hooks/useLaunchpadData";
import { getAppConfig } from "./config";

// The admin area is code-split: standard users never download it.
const AdminApp = lazy(() => import("../admin/AdminApp"));

export default function App() {
  const { themeMode, isDarkMode, setThemeMode } = useTheme();
  const { user } = useUser();
  const { isAdmin } = useCallerIdentity(user);
  const { services, onTogglePin } = useLaunchpadData(user);
  const config = getAppConfig();

  return (
    <Toaster>
      <main className="w-full pt-(--top-banner-height,0px) pb-(--bottom-banner-height,0px)">
        <Banner position="top" config={config?.banners?.top} />
        <Header
          isDarkMode={isDarkMode}
          themeMode={themeMode}
          onThemeChange={setThemeMode}
          user={user}
          onSignIn={() => signIn()}
          onSignOut={() => signOut()}
          logoSrc={config?.logoUrl || undefined}
          logoSrcDark={config?.logoUrlDark || undefined}
          isAdmin={isAdmin}
        />

        <Routes>
          <Route path="/" element={<Content services={services} onTogglePin={onTogglePin} />} />
          <Route
            path="/admin/*"
            element={
              <Suspense fallback={null}>
                <AdminApp user={user} />
              </Suspense>
            }
          />
          <Route path="*" element={<Content services={services} onTogglePin={onTogglePin} />} />
        </Routes>
        <Banner position="bottom" config={config?.banners?.bottom} />
      </main>
    </Toaster>
  );
}
