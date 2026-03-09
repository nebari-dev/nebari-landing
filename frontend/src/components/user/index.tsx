import {
  useEffect,
  useId,
  useRef,
  useState
} from "react";

import type {
  ReactNode
} from "react";

import {
  Button
} from "@trussworks/react-uswds";

import {
  ChevronDown,
  LogOut
} from "lucide-react";

import type {
  User
} from "../../auth/user";

import "./index.scss";

function capitalizeFirst(value: string): string {
  const v = value.trim();
  if (!v) return v;
  return v.charAt(0).toUpperCase() + v.slice(1);
}

export default function HeaderUserMenu(props: HeaderUserMenu.Props): ReactNode {

  const {
    user,
    onSignOut
  } = props;

  const menuId = useId();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const signOutRef = useRef<HTMLButtonElement | null>(null);

  const [open, setOpen] = useState(false);

  const initials = getInitials(user?.name);

  useEffect(() => {
    if (!open) return;

    // Focus the first menu action when opening
    signOutRef.current?.focus();

    const onPointerDown = (e: PointerEvent) => {
      const root = rootRef.current;
      if (!root) return;

      if (!root.contains(e.target as Node)) {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };

    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);

    return () => {
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const handleToggle = () => {
    setOpen((prev) => !prev);
  };

  const handleSignOut = () => {
    setOpen(false);
    onSignOut?.();
  };

  return (
    <div
      ref={rootRef}
    >
      <Button
        type="button"
        unstyled
        className="text-no-underline hover:text-no-underline"
        onClick={handleToggle}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        aria-label={`Account menu for ${user.name}`}
        ref={triggerRef}
      >
        <span className="app-userMenu__avatar" aria-hidden="true">
          {initials}
        </span>

        <span className="app-userMenu__name">
          {capitalizeFirst(user.name)}
        </span>

        <span
          className={`app-userMenu__chevronButton ${open ? "is-open" : ""}`}
          aria-hidden="true"
        >
          <ChevronDown size={15} className="app-userMenu__chevronIcon" />
        </span>
      </Button>

      {open ? (
        <ul
          id={menuId}
          className="app-userMenu__menu"
          role="menu"
          aria-label="Account actions"
        >
          <li role="none">
            <button
              type="button"
              role="menuitem"
              className="app-userMenu__menuItem"
              onClick={handleSignOut}
              ref={signOutRef}
            >
              <LogOut
                size={15}
                className="app-userMenu__menuItemIcon"
                aria-hidden="true"
              />
              <span className="app-userMenu__menuItemText">
                Logout
              </span>
            </button>
          </li>
        </ul>
      ) : null}
    </div>
  );
}

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);

  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase();

  const first = parts[0].slice(0, 1).toUpperCase();
  const last = parts[parts.length - 1].slice(0, 1).toUpperCase();
  return `${first}${last}`;
}

export namespace HeaderUserMenu {

  export
  type Props = {
    user: User;
    onSignOut?: () => void;
  };
}
