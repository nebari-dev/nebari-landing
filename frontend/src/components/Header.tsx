import { Menu as MenuPrimitive } from "@base-ui/react/menu";
import { ChevronDown, LogOut, Monitor, Moon, Sun } from "lucide-react";
import type { ReactNode } from "react";
import builtInLogoDark from "../assets/nebari-logo_dark.svg";
import builtInLogoLight from "../assets/nebari-logo_light.svg";
import { isThemeMode, type ThemeMode } from "../hooks/use-theme-preference";
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
    logoSrc: logoSrcProp,
    logoSrcDark: logoSrcDarkProp,
  } = props;

  // Dark mode prefers the dark logo, then the light/general custom logo, then
  // the built-in dark logo. Light mode uses the custom logo or the built-in.
  const logoSrc = isDarkMode
    ? (logoSrcDarkProp ?? logoSrcProp ?? builtInLogoDark)
    : (logoSrcProp ?? builtInLogoLight);

  const initials = getUserInitials(user?.name, user?.email);

  return (
    <NavigationMenu className="h-14 justify-between border-header-border bg-header-background pl-4 text-header-foreground">
      <MenuBarBrand href={homeHref} aria-label="Go to homepage">
        <img src={logoSrc} alt="Nebari" className="h-8 w-auto" />
      </MenuBarBrand>

      <MenuBarActions className="gap-2">
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
