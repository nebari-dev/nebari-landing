import { Header } from "../components/Header"
import { Content } from "../components/Content"
import { signOut } from "../auth/keycloak"
import { useUser } from "../auth/user"

import { useThemePreference } from "../hooks/useThemePreference"
import { useLaunchpadData } from "../hooks/useLaunchpadData"
import { getAppConfig } from "./config"


export default function App() {
  const { isDarkMode, toggleTheme } = useThemePreference()
  const { user } = useUser()
  const {
    services,
    notifications,
    onNotificationsViewed,
    onTogglePin,
  } = useLaunchpadData(user)

  const config = getAppConfig()

  return (
    <main className="w-full">
      <Header
        isDarkMode={isDarkMode}
        onToggleTheme={toggleTheme}
        user={user}
        onSignOut={() => signOut()}
        notifications={notifications}
        onNotificationsViewed={onNotificationsViewed}
        logoSrc={config?.logoUrl || undefined}
      />

      <Content services={services} onTogglePin={onTogglePin} />
    </main>
  )
}
