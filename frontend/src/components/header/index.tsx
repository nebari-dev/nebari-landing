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
  ExternalLink, Moon, Sun, Bell
} from "lucide-react";

import "./index.scss";

export default function Header(props: HeaderProps): ReactNode {

  const {
    homeHref = "/",
    docsHref = "/docs",
    githubHref = "https://github.com/trussworks/react-uswds",
    onSignIn,
    signInLabel = "Sign in",
    isDarkMode = false,
    onToggleTheme
  } = props

  const logoSrc = isDarkMode ? logoUrlDark : logoUrlLight;

  return (
    <USWDSHeader
      basic
      className="app-header"
    >
      <div className="app-header__row">
        <div className="app-header__left">
          <a href={homeHref} className="app-header__brand">
            <img
              src={logoSrc}
              className="app-header__logo"
            />
          </a>

          <nav className="app-header__nav">
            <a className="app-header__link" href={docsHref}>
              Docs <ExternalLink size={13} className="app-header__linkIcon" />
            </a>
            <a
              className="app-header__link"
              href={githubHref}
              target="_blank"
              rel="noreferrer"
            >
              GitHub <ExternalLink size={13} className="app-header__linkIcon" />
            </a>
          </nav>
        </div>

        <div className="app-header__right">
          <Button
            type="button"
            outline
            className="app-header__themeButton"
            onClick={onToggleTheme}
          >
            {isDarkMode ? (
              <Sun size={16} className="app-header__buttonIcon" />
            ) : (
              <Moon size={16} className="app-header__buttonIcon" />
            )}
          </Button>

          {/* TODO: Get the notification modal working */}
          <Button
            type="button"
            outline
            className="app-header__themeButton"
            // onClick={onNotifications}
          >
            <Bell size={16} className="app-header__buttonIcon" />
          </Button>

          <Button type="button" onClick={onSignIn}>
            {signInLabel}
          </Button>
        </div>
      </div>
    </USWDSHeader>
  );
}

export type HeaderProps = {
  homeHref?: string;
  docsHref?: string;
  githubHref?: string;
  onSignIn?: () => void;
  signInLabel?: string;
  isDarkMode?: boolean;
  onToggleTheme?: () => void;
};
