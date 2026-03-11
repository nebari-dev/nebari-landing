import {
  useEffect,
  useId,
  useMemo,
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
  const panelRef = useRef<HTMLDivElement | null>(null);

  const [open, setOpen] = useState(false);

  const sortedNotifications = useMemo(() => {
    return [...notifications].sort((a, b) => {
      if (a.read !== b.read) {
        return a.read ? 1 : -1;
      }

      return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
    });
  }, [notifications]);

  const unreadCount = notifications.filter((notification) => !notification.read).length;

  useEffect(() => {
    if (!open) return;

    const markVisibleNotificationsRead = () => {
      const panel = panelRef.current;
      if (!panel) return;

      const panelRect = panel.getBoundingClientRect();

      const visibleUnreadIds = sortedNotifications
        .filter((notification) => !notification.read)
        .filter((notification) => {
          const element = panel.querySelector<HTMLElement>(
            `[data-notification-id="${notification.id}"]`
          );

          if (!element) return false;

          const rect = element.getBoundingClientRect();

          return rect.top < panelRect.bottom && rect.bottom > panelRect.top;
        })
        .map((notification) => notification.id);

      if (visibleUnreadIds.length > 0) {
        onNotificationsViewed?.(visibleUnreadIds);
      }
    };

    markVisibleNotificationsRead();

    const panel = panelRef.current;
    if (!panel) return;

    panel.addEventListener("scroll", markVisibleNotificationsRead);

    return () => {
      panel.removeEventListener("scroll", markVisibleNotificationsRead);
    };
  }, [open, sortedNotifications, onNotificationsViewed]);

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
        className="app-header__themeButton app-notifications__button usa-button--small padding-0"
        aria-label={
          unreadCount > 0
            ? `Open notifications, ${unreadCount} unread`
            : "Open notifications"
        }
        title="Open notifications"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((prev) => !prev)}
        ref={triggerRef}
      >
        <Bell size={15} className="app-header__buttonIcon" aria-hidden="true" />

        {unreadCount > 0 ? (
          <span className="app-notifications__badge" aria-hidden="true">
            {unreadCount > 99 ? "99+" : unreadCount}
          </span>
        ) : null}
      </Button>

      {open ? (
        <div
          id={panelId}
          ref={panelRef}
          className="app-notifications__panel"
          role="region"
          aria-label="Notifications"
        >
          <NotificationList notifications={sortedNotifications} />
        </div>
      ) : null}
    </div>
  );
}
