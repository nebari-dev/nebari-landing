import type {
  ReactNode
} from "react";

import {
  Header as USWDSHeader, Button
} from "@trussworks/react-uswds";

import
  logoUrlDark
from "../../assets/nebari-logo_dark.svg";

import
  logoUrlLight
from "../../assets/nebari-logo_light.svg";

import {
  Moon, Sun
} from "lucide-react";

import type {
  User
} from "../../auth/user";

import
  HeaderUserMenu
from "../user";

import
  HeaderNotificationsMenu
from "../notifications";

import type {
  Notification
} from "../../api/notifications"; 

import "./index.scss";

export default function Header(props: HeaderProps): ReactNode {
  const {
    homeHref = "/",
    isDarkMode = false,
    onToggleTheme,
    user,
    onSignOut,
    notifications,
    onNotificationsViewed,
  } = props;

  const logoSrc = isDarkMode ? logoUrlDark : logoUrlLight;

  return (
    <USWDSHeader basic className="app-header">
      <div className="app-header__row">
        <div className="app-header__left">
          <a
            href={homeHref}
            className="app-header__brand"
            aria-label="Go to homepage"
          >
            <img
              src={logoSrc}
              className="app-header__logo"
              alt="Nebari"
            />
          </a>
        </div>

        <div className="app-header__right">
          <Button
            type="button"
            outline
            className="app-header__themeButton usa-button--small padding-0"
            onClick={onToggleTheme}
            aria-label={isDarkMode ? "Switch to light mode" : "Switch to dark mode"}
            aria-pressed={isDarkMode}
            title={isDarkMode ? "Switch to light mode" : "Switch to dark mode"}
          >
            {isDarkMode ? (
              <Sun size={15} className="app-header__buttonIcon" aria-hidden="true" />
            ) : (
              <Moon size={15} className="app-header__buttonIcon" aria-hidden="true" />
            )}
          </Button>

          <HeaderNotificationsMenu
            notifications={notifications}
            onNotificationsViewed={onNotificationsViewed}
          />

          {user ? (
            <HeaderUserMenu
              user={user}
              onSignOut={onSignOut}
            />
          ) : null}
        </div>
      </div>
    </USWDSHeader>
  );
}

export type HeaderProps = {
  homeHref?: string;
  docsHref?: string;
  githubHref?: string;
  isDarkMode?: boolean;
  onToggleTheme?: () => void;
  user?: User | null;
  onSignOut?: () => void;
  notifications: Notification[];
  onNotificationsViewed?: (ids: string[]) => void | Promise<void>;
};
