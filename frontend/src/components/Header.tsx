import { Menu as MenuPrimitive } from "@base-ui/react/menu";
import { Bell, ChevronDown, LogOut, Monitor, Moon, Sun } from "lucide-react";
import type { ReactNode } from "react";
import builtInLogoDark from "../assets/nebari-logo_dark.svg";
import builtInLogoLight from "../assets/nebari-logo_light.svg";
import { isThemeMode, type ThemeMode } from "../hooks/useThemePreference";
import { cn } from "../lib/utils";
import { Avatar, AvatarFallback, AvatarImage } from "./ui/avatar";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import { MenuBarActions, MenuBarBrand, NavigationMenu } from "./ui/navigation-menu";

type Notification = {
  id: string;
  title: string;
  message: string;
  createdAt: string;
  image?: string;
  read?: boolean;
};

type User = {
  name?: string;
  email?: string;
  image?: string;
};

export type HeaderProps = {
  homeHref?: string;
  isDarkMode?: boolean;
  themeMode?: ThemeMode;
  onThemeChange?: (mode: ThemeMode) => void;
  user?: User | null;
  onSignIn?: () => void;
  onSignOut?: () => void;
  notifications?: Notification[];
  onNotificationsViewed?: (ids: string[]) => void | Promise<void>;
  logoSrc?: string;
  logoSrcDark?: string;
};

export function Header(props: HeaderProps): ReactNode {
  const {
    homeHref = "/",
    isDarkMode = false,
    themeMode = "system",
    onThemeChange,
    user,
    onSignIn,
    onSignOut,
    notifications = [],
    onNotificationsViewed,
    logoSrc: logoSrcProp,
    logoSrcDark: logoSrcDarkProp,
  } = props;

  const unreadNotifications = notifications.filter((item) => !item.read);
  const unreadCount = unreadNotifications.length;
  // Dark mode prefers the dark logo, then the light/general custom logo, then
  // the built-in dark logo. Light mode uses the custom logo or the built-in.
  const logoSrc = isDarkMode
    ? (logoSrcDarkProp ?? logoSrcProp ?? builtInLogoDark)
    : (logoSrcProp ?? builtInLogoLight);

  const initials = getUserInitials(user?.name, user?.email);

  const handleNotificationsOpen = () => {
    if (!onNotificationsViewed) return;

    const unreadIds = unreadNotifications.map((item) => item.id);
    if (unreadIds.length > 0) {
      void onNotificationsViewed(unreadIds);
    }
  };

  return (
    <NavigationMenu className="h-14 justify-between border-header-border bg-header-background pl-4 text-header-foreground">
      <MenuBarBrand href={homeHref} aria-label="Go to homepage">
        <img src={logoSrc} alt="Nebari" className="h-8 w-auto" />
      </MenuBarBrand>

      <MenuBarActions className="gap-2">
        <DropdownMenu modal={false} onOpenChange={(open) => open && handleNotificationsOpen()}>
          <DropdownMenuTrigger
            variant="ghost"
            className="relative w-8 px-0 hover:bg-header-action-hover hover:no-underline focus-visible:ring-offset-0 active:bg-header-action-hover"
            aria-label="Notifications"
          >
            <Bell />
            {unreadCount > 0 ? (
              <span className="absolute -right-0.5 -top-0.5 z-10 flex h-4 min-w-4 items-center justify-center rounded-full bg-notification-badge px-1 pt-px text-[9px] font-semibold leading-none text-white tabular-nums">
                {unreadCount}
              </span>
            ) : null}
          </DropdownMenuTrigger>

          <DropdownMenuPortal>
            <DropdownMenuContent
              align="end"
              className="max-h-(--available-height) w-[552px] overflow-y-auto p-0"
            >
              {notifications.length > 0 ? (
                notifications.map((notification) => (
                  <DropdownMenuItem
                    key={notification.id}
                    className="items-start gap-4 rounded-none border-b px-4 py-4 whitespace-normal last:border-b-0"
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
                <div className="px-4 py-4 text-sm text-muted-foreground">No notifications</div>
              )}
            </DropdownMenuContent>
          </DropdownMenuPortal>
        </DropdownMenu>

        {user ? (
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger
              variant="ghost"
              aria-label="Account menu"
              className="h-auto px-2.5 py-1 hover:bg-header-action-hover hover:no-underline focus-visible:ring-offset-0 active:bg-header-action-hover data-[popup-open]:bg-header-action-hover data-[popup-open]:no-underline"
            >
              <Avatar>
                {user.image ? <AvatarImage src={user.image} alt={user.name ?? "User"} /> : null}
                <AvatarFallback className="bg-primary font-semibold text-primary-foreground">
                  {initials}
                </AvatarFallback>
              </Avatar>

              <span>{user.name ?? user.email ?? "Account"}</span>

              <ChevronDown />
            </DropdownMenuTrigger>

            <DropdownMenuPortal>
              <DropdownMenuContent align="end" className="w-[248px] p-2">
                <div className="border-b px-1.5 pb-2">
                  <p className="text-sm font-medium text-foreground">{user.name ?? "Signed in"}</p>
                  {user.email ? (
                    <p className="text-xs text-muted-foreground">{user.email}</p>
                  ) : null}
                </div>

                <div className="py-2">
                  <MenuPrimitive.RadioGroup
                    aria-label="Theme"
                    value={themeMode}
                    onValueChange={(value) => {
                      if (isThemeMode(value)) onThemeChange?.(value);
                    }}
                    className="flex h-[34px] items-center gap-1 rounded-md bg-muted p-1"
                  >
                    <ThemeOption value="light" label="Light mode" text="Light">
                      <Sun className="h-4 w-4" />
                    </ThemeOption>

                    <ThemeOption value="dark" label="Dark mode" text="Dark">
                      <Moon className="h-4 w-4" />
                    </ThemeOption>

                    <ThemeOption value="system" label="System theme" text="System">
                      <Monitor className="h-4 w-4" />
                    </ThemeOption>
                  </MenuPrimitive.RadioGroup>
                </div>

                <DropdownMenuSeparator />

                <DropdownMenuItem
                  className="leading-5 text-sign-out-foreground data-[highlighted]:text-sign-out-foreground"
                  onClick={() => onSignOut?.()}
                >
                  <LogOut className="size-4 shrink-0" aria-hidden="true" />
                  Sign out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenuPortal>
          </DropdownMenu>
        ) : (
          <Button type="button" onClick={() => onSignIn?.()}>
            Sign in
          </Button>
        )}
      </MenuBarActions>
    </NavigationMenu>
  );
}

function ThemeOption({
  value,
  label,
  text,
  children,
}: {
  value: ThemeMode;
  label: string;
  text: string;
  children: ReactNode;
}): ReactNode {
  return (
    <MenuPrimitive.RadioItem
      value={value}
      aria-label={label}
      title={label}
      closeOnClick={false}
      className={cn(
        "flex h-auto flex-1 cursor-pointer items-center justify-center gap-1 rounded-sm border border-transparent px-1.5 py-0.5 text-sm font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring",
        "text-muted-foreground-strong hover:text-foreground",
        "data-checked:border-border-strong data-checked:bg-card data-checked:text-foreground data-checked:shadow-[0_1px_3px_0_rgba(0,0,0,0.10)]",
      )}
    >
      {children}
      <span>{text}</span>
    </MenuPrimitive.RadioItem>
  );
}

function getUserInitials(name?: string, email?: string) {
  if (name) {
    const parts = name.trim().split(/\s+/).filter(Boolean);
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
    }
    if (parts.length === 1) {
      return parts[0].slice(0, 2).toUpperCase();
    }
  }

  if (email) {
    return email.slice(0, 2).toUpperCase();
  }

  return "U";
}
