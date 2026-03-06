import {
  type ReactNode, useRef, useState, useEffect
} from "react";

import { Header as USWDSHeader, Button } from "@trussworks/react-uswds";

import logoUrlDark from "../../assets/nebari-logo_dark.svg";
import logoUrlLight from "../../assets/nebari-logo_light.svg";

import { ExternalLink, Moon, Sun, Bell, ChevronDown } from "lucide-react";

import type { User } from "../../auth/user";
import { getInitials } from "../../auth/user";
import { signIn, signOut } from "../../auth/keycloak";

import "./index.scss";

const NOTIFICATION_COUNT = 3;

function ThemeToggle({ isDarkMode, onToggle }: { isDarkMode: boolean; onToggle?: () => void }) {
  return (
    <Button
      type="button"
      outline
      className="app-header__iconBtn"
      onClick={onToggle}
      title={isDarkMode ? "Switch to light mode" : "Switch to dark mode"}
    >
      {isDarkMode
        ? <Sun size={16} className="app-header__btnIcon" />
        : <Moon size={16} className="app-header__btnIcon" />}
    </Button>
  );
}

function NotificationBell() {
  return (
    <div className="app-header__notifWrap">
      <Button type="button" outline className="app-header__iconBtn" title="Notifications">
        <Bell size={16} className="app-header__btnIcon" />
      </Button>
      <span className="app-header__badge" aria-label={`${NOTIFICATION_COUNT} notifications`}>
        {NOTIFICATION_COUNT}
      </span>
    </div>
  );
}

function UserMenu({ user }: { user: User }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  return (
    <div className="app-header__userMenu" ref={ref}>
      <button
        type="button"
        className="app-header__userBtn"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <span className="app-header__avatar" aria-hidden="true">
          {getInitials(user.name)}
        </span>
        <span className="app-header__userName">{user.name}</span>
        <ChevronDown size={14} className={`app-header__chevron${open ? " app-header__chevron--open" : ""}`} />
      </button>

      {open && (
        <div className="app-header__dropdown" role="menu">
          {user.email && (
            <span className="app-header__dropdownEmail">{user.email}</span>
          )}
          <button
            type="button"
            role="menuitem"
            className="app-header__dropdownItem"
            onClick={signOut}
          >
            Sign out
          </button>
        </div>
      )}
    </div>
  );
}

export default function Header(props: HeaderProps): ReactNode {
  const {
    homeHref = "/",
    docsHref = "/docs",
    githubHref = "https://github.com/trussworks/react-uswds",
    isDarkMode = false,
    onToggleTheme,
    user,
  } = props;

  const logoSrc = isDarkMode ? logoUrlDark : logoUrlLight;

  return (
    <USWDSHeader basic className="app-header">
      <div className="app-header__row">
        <div className="app-header__left">
          <a href={homeHref} className="app-header__brand">
            <img src={logoSrc} className="app-header__logo" alt="Nebari" />
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
          <ThemeToggle isDarkMode={isDarkMode} onToggle={onToggleTheme} />

          {user ? (
            <>
              <NotificationBell />
              <UserMenu user={user} />
            </>
          ) : (
            <Button type="button" onClick={signIn}>
              Sign in
            </Button>
          )}
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
};
