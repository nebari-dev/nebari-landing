import {
  type ReactNode, useRef, useState, useEffect
} from "react";

import { Header as USWDSHeader, Button } from "@trussworks/react-uswds";

import logoUrlDark from "../../assets/nebari-logo_dark.svg";
import logoUrlLight from "../../assets/nebari-logo_light.svg";

import { ExternalLink, Moon, Sun, Bell, ChevronDown, LogOut } from "lucide-react";

import type { User } from "../../auth/user";
import { getInitials } from "../../auth/user";
import { signIn, signOut } from "../../auth/keycloak";

import "./index.scss";

// ── Types ─────────────────────────────────────────────────────────────────────

type OpenPanel = "notif" | "user" | null;

const MOCK_NOTIFICATIONS = [
  {
    id: 1,
    title: "New service available",
    subtitle: "JupyterHub has been upgraded to version 4.2 and is ready to use.",
    initials: "JH",
  },
  {
    id: 2,
    title: "Access request approved",
    subtitle: "Your request for MLflow has been approved by the admin.",
    initials: "ML",
  },
  {
    id: 3,
    title: "Scheduled maintenance",
    subtitle: "The cluster will be restarted tonight at 02:00 UTC.",
    initials: "OP",
  },
];

// ── Shared click-away hook ────────────────────────────────────────────────────

function useClickAway(ref: React.RefObject<HTMLElement | null>, onClose: () => void, active: boolean) {
  useEffect(() => {
    if (!active) return;
    function handler(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [active, ref, onClose]);
}

// ── Theme toggle ─────────────────────────────────────────────────────────────

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

// ── Notification bell + panel ─────────────────────────────────────────────────

function NotificationBell({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useClickAway(ref, onToggle, open);

  return (
    <div className="app-header__notifWrap" ref={ref}>
      <Button
        type="button"
        outline
        className="app-header__iconBtn"
        onClick={onToggle}
        title="Notifications"
        aria-expanded={open}
        aria-haspopup="true"
      >
        <Bell size={16} className="app-header__btnIcon" />
      </Button>
      <span className="app-header__badge" aria-label={`${MOCK_NOTIFICATIONS.length} notifications`}>
        {MOCK_NOTIFICATIONS.length}
      </span>

      {open && (
        <div className="app-header__notifPanel" role="dialog" aria-label="Notifications">
          {MOCK_NOTIFICATIONS.map((n, i) => (
            <div
              key={n.id}
              className={`app-header__notifItem${i < MOCK_NOTIFICATIONS.length - 1 ? " app-header__notifItem--divided" : ""}`}
            >
              <div className="app-header__notifIcon" aria-hidden="true">
                {n.initials}
              </div>
              <div className="app-header__notifText">
                <div className="app-header__notifTitle">{n.title}</div>
                <div className="app-header__notifSubtitle">{n.subtitle}</div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// ── User menu ─────────────────────────────────────────────────────────────────

function UserMenu({ user, open, onToggle }: { user: User; open: boolean; onToggle: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  useClickAway(ref, onToggle, open);

  return (
    <div className="app-header__userMenu" ref={ref}>
      <button
        type="button"
        className="app-header__userBtn"
        onClick={onToggle}
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <span className="app-header__avatar" aria-hidden="true">
          {getInitials(user.name)}
        </span>
        <span className="app-header__userName">{user.name}</span>
        <ChevronDown
          size={16}
          className={`app-header__chevron${open ? " app-header__chevron--open" : ""}`}
        />
      </button>

      {open && (
        <div className="app-header__userDropdown" role="menu">
          {user.email && (
            <span className="app-header__dropdownEmail">{user.email}</span>
          )}
          <button
            type="button"
            role="menuitem"
            className="app-header__menuItem app-header__menuItem--danger"
            onClick={signOut}
          >
            <LogOut size={15} className="app-header__menuIcon" />
            Sign out
          </button>
        </div>
      )}
    </div>
  );
}

// ── Header ────────────────────────────────────────────────────────────────────

export default function Header(props: HeaderProps): ReactNode {
  const {
    homeHref = "/",
    docsHref = "/docs",
    githubHref = "https://github.com/trussworks/react-uswds",
    isDarkMode = false,
    onToggleTheme,
    user,
  } = props;

  const [openPanel, setOpenPanel] = useState<OpenPanel>(null);

  const toggle = (panel: OpenPanel) =>
    setOpenPanel((cur) => (cur === panel ? null : panel));

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
              <NotificationBell
                open={openPanel === "notif"}
                onToggle={() => toggle("notif")}
              />
              <UserMenu
                user={user}
                open={openPanel === "user"}
                onToggle={() => toggle("user")}
              />
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

