import { useState, useEffect, useCallback } from "react";

import Header from "../components/header";
import Content from "../components/content";

import { signOut } from "../auth/keycloak";
import { useUser } from "../auth/user";

import { listServices, type Service } from "../api/listServices";
import {
  listNotifications,
  markNotificationRead,
  type Notification
} from "../api/notifications";


export default function App() {
  const [isDarkMode, setIsDarkMode] = useState(false);
  const [services, setServices] = useState<Service[] | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const { user } = useUser();

  useEffect(() => {
    listServices()
      .then((data) => {
        setServices(data);
      })
      .catch((err) => {
        console.error("listServices failed", err);
      });

    listNotifications()
      .then((data) => {
        setNotifications(data);
      })
      .catch((err) => {
        console.error("listNotifications failed", err);
      });
  }, []);

  const handleNotificationsViewed = useCallback(async (ids: string[]) => {
    if (ids.length === 0) return;

    setNotifications((prev) =>
      prev.map((notification) =>
        ids.includes(notification.id)
          ? { ...notification, read: true }
          : notification
      )
    );

    try {
      await Promise.all(ids.map((id) => markNotificationRead(id)));
    } catch (err) {
      console.error("markNotificationRead failed", err);

      setNotifications((prev) =>
        prev.map((notification) =>
          ids.includes(notification.id)
            ? { ...notification, read: false }
            : notification
        )
      );
    }
  }, []);

  return (
    <div className={isDarkMode ? "app-shell app-shell--dark" : "app-shell app-shell--light"}>
      <Header
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode((prev) => !prev)}
        user={user}
        onSignOut={() => signOut()}
        notifications={notifications}
        onNotificationsViewed={handleNotificationsViewed}
      />
      <Content services={services} />
    </div>
  );
}
