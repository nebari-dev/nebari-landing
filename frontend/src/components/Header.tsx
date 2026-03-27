import { Bell, ChevronDown, Moon, Sun } from "lucide-react"
import type { ReactNode } from "react"

import { Button } from "./ui/button"
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "./ui/avatar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu"

import logoUrlDark from "../assets/nebari-logo_dark.svg"
import logoUrlLight from "../assets/nebari-logo_light.svg"

type Notification = {
  id: string
  title: string
  message: string
  createdAt: string
  image?: string
  read?: boolean
}

type User = {
  name?: string
  email?: string
  image?: string
}

export type HeaderProps = {
  homeHref?: string
  isDarkMode?: boolean
  onToggleTheme?: () => void
  user?: User | null
  onSignIn?: () => void
  onSignOut?: () => void
  notifications?: Notification[]
  onNotificationsViewed?: (ids: string[]) => void | Promise<void>
  logoSrc?: string
}

export function Header(props: HeaderProps): ReactNode {
  const {
    homeHref = "/",
    isDarkMode = false,
    onToggleTheme,
    user,
    onSignIn,
    onSignOut,
    notifications = [],
    onNotificationsViewed,
    logoSrc: logoSrcProp,
  } = props

  const unreadNotifications = notifications.filter((item) => !item.read)
  const unreadCount = unreadNotifications.length
  const logoSrc = logoSrcProp ?? (isDarkMode ? logoUrlDark : logoUrlLight)

  const initials = getUserInitials(user?.name, user?.email)

  const handleNotificationsOpen = () => {
    if (!onNotificationsViewed) return

    const unreadIds = unreadNotifications.map((item) => item.id)
    if (unreadIds.length > 0) {
      void onNotificationsViewed(unreadIds)
    }
  }

  return (
    <header className="flex h-[60px] w-full items-center justify-between border-b bg-background px-10">
      <div className="flex items-center">
        <a href={homeHref} className="flex items-center" aria-label="Go to homepage">
          <img src={logoSrc} alt="Nebari" className="h-8 w-auto" />
        </a>
      </div>

      <div className="flex items-center gap-3">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="relative h-9 w-9 rounded-[8px] border border-border bg-background text-muted-foreground transition-none hover:bg-accent"
          aria-label={isDarkMode ? "Switch to light mode" : "Switch to dark mode"}
          aria-pressed={isDarkMode}
          title={isDarkMode ? "Switch to light mode" : "Switch to dark mode"}
          onClick={onToggleTheme}
        >
          {isDarkMode ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
        </Button>

        <DropdownMenu onOpenChange={(open) => open && handleNotificationsOpen()}>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="relative h-9 w-9 rounded-[8px] border border-border bg-background text-muted-foreground transition-none hover:bg-accent"
              aria-label="Notifications"
            >
              <Bell className="h-5 w-5" />
              {unreadCount > 0 ? (
                <span className="absolute -right-1 -top-1 z-10 flex h-5 min-w-5 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-semibold text-white">
                  {unreadCount}
                </span>
              ) : null}
            </Button>
          </DropdownMenuTrigger>

          <DropdownMenuContent align="end" className="w-[552px] p-0">
            {notifications.length > 0 ? (
              notifications.map((notification) => (
                <DropdownMenuItem
                  key={notification.id}
                  className="flex cursor-default items-start gap-4 whitespace-normal border-b px-4 py-4 last:border-b-0"
                >
                  <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-muted">
                    {notification.image ? (
                      <img
                        src={notification.image}
                        alt=""
                        aria-hidden="true"
                        className="h-9 w-9 object-contain"
                      />
                    ) : null}
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="flex items-start justify-between gap-3">
                      <span className="text-[15px] font-semibold leading-6 text-foreground">
                        {notification.title}
                      </span>

                      {!notification.read ? (
                        <span className="mt-2 h-2 w-2 shrink-0 rounded-full bg-primary" />
                      ) : null}
                    </div>

                    <p className="text-(--text-secondary) text-sm leading-7">
                      {notification.message}
                    </p>
                  </div>
                </DropdownMenuItem>
              ))
            ) : (
              <div className="px-4 py-4 text-sm text-muted-foreground">
                No notifications
              </div>
            )}
          </DropdownMenuContent>
        </DropdownMenu>

        {user ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                className="flex items-center gap-3 rounded-md px-1 py-1 transition-none hover:bg-accent focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
              >
                <Avatar className="h-8 w-8">
                  {user.image ? <AvatarImage src={user.image} alt={user.name ?? "User"} /> : null}
                  <AvatarFallback className="bg-violet-600 text-sm font-semibold text-white">
                    {initials}
                  </AvatarFallback>
                </Avatar>

                <span className="text-sm font-medium text-foreground">
                  {user.name ?? user.email ?? "Account"}
                </span>

                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              </button>
            </DropdownMenuTrigger>

            <DropdownMenuContent align="end" className="w-56">
              <div className="border-b px-3 py-2">
                <p className="text-sm font-medium text-foreground">
                  {user.name ?? "Signed in"}
                </p>
                {user.email ? (
                  <p className="text-xs text-muted-foreground">{user.email}</p>
                ) : null}
              </div>

              <DropdownMenuItem
                className="cursor-pointer focus:bg-accent focus:text-accent-foreground focus:outline-none focus:ring-[3px] focus:ring-ring/50"
                onClick={() => onSignOut?.()}
              >
                Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : (
          <Button type="button" onClick={() => onSignIn?.()}>
            Sign in
          </Button>
        )}
      </div>
    </header>
  )
}

function getUserInitials(name?: string, email?: string) {
  if (name) {
    const parts = name.trim().split(/\s+/).filter(Boolean)
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[1][0]}`.toUpperCase()
    }
    if (parts.length === 1) {
      return parts[0].slice(0, 2).toUpperCase()
    }
  }

  if (email) {
    return email.slice(0, 2).toUpperCase()
  }

  return "U"
}
