import {
  useEffect,
  useId,
  useRef,
  useState,
} from "react";

import type { ReactNode } from "react";
import type { Notification } from "../../api/notifications";

import { Button } from "@trussworks/react-uswds";
import { Bell } from "lucide-react";

import NotificationList from "./notificationsList";

import "./index.scss";

type HeaderNotificationsMenuProps = {
  notifications: Notification[];
  onNotificationsViewed?: (ids: string[]) => void | Promise<void>;
};

export default function HeaderNotificationsMenu(
  props: HeaderNotificationsMenuProps
): ReactNode {
  const { notifications, onNotificationsViewed } = props;

  const panelId = useId();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;

    const unreadIds = notifications
      .filter((notification) => !notification.read)
      .map((notification) => notification.id);

    if (unreadIds.length > 0) {
      onNotificationsViewed?.(unreadIds);
    }
  }, [open, notifications, onNotificationsViewed]);

  useEffect(() => {
    if (!open) return;

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

  return (
    <div className="app-notifications" ref={rootRef}>
      <Button
        type="button"
        outline
        className="app-header__themeButton usa-button--small padding-0"
        aria-label="Open notifications"
        title="Open notifications"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((prev) => !prev)}
        ref={triggerRef}
      >
        <Bell size={15} className="app-header__buttonIcon" aria-hidden="true" />
      </Button>

      {open ? (
        <div
          id={panelId}
          className="app-notifications__panel"
          role="region"
          aria-label="Notifications"
        >
          <NotificationList notifications={notifications} />
        </div>
      ) : null}
    </div>
  );
}
