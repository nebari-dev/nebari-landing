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
  ExternalLink, Moon, Sun
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

import "./index.scss";

export default function Header(props: HeaderProps): ReactNode {
  const {
    homeHref = "/",
    docsHref = "/docs",
    githubHref = "https://github.com/trussworks/react-uswds",
    isDarkMode = false,
    onToggleTheme,
    user,
    onSignOut
  } = props;

  const logoSrc = isDarkMode ? logoUrlDark : logoUrlLight;

  const notifications = [
    {
      id: "n-1",
      image: "",
      title: "Service updated",
      description: "Payments service completed a configuration update."
    },
    {
      id: "n-2",
      image: "",
      title: "Access approved",
      description: "Your request for Developer Portal access was approved."
    },
    {
      id: "n-3",
      image: "",
      title: "New release",
      description: "Analytics service is now available in Launchpad."
    }
  ];

  return (
    <USWDSHeader
      basic
      className="app-header"
    >
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

          <nav className="app-header__nav" aria-label="Primary navigation">
            <a
              className="app-header__link"
              href={docsHref}
              aria-label="Open documentation"
            >
              Docs{" "}
              <ExternalLink size={13} className="app-header__linkIcon" aria-hidden="true" />
            </a>

            <a
              className="app-header__link"
              href={githubHref}
              target="_blank"
              rel="noreferrer"
              aria-label="Open GitHub repository in a new tab"
            >
              GitHub{" "}
              <ExternalLink size={13} className="app-header__linkIcon" aria-hidden="true" />
            </a>
          </nav>
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

          <HeaderNotificationsMenu notifications={notifications} />

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
};
