import { useCallback, useEffect, useMemo, useState } from "react";
import Header from "../components/header";
import Content from "../components/content";
import { signOut } from "../auth/keycloak";
import { useUser } from "../auth/user";

import { listServices, type Service } from "../api/listServices";
import {
  listNotifications,
  markNotificationRead,
  type Notification,
} from "../api/notifications";
import { createWebSocketClient } from "../api/ws";
import { mapService } from "../api/mapServices";

type BackendSocketService = {
  uid: string;
  name: string;
  namespace: string;
  displayName: string;
  description: string;
  url: string;
  icon: string;
  category: string;
  priority: number;
  pinned: boolean;
  visibility: string;
  health: {
    status: string;
    lastCheck: string;
    message?: string;
  };
};

type AppSocketMessage = {
  type: "added" | "modified" | "deleted";
  service: BackendSocketService;
};

export default function App() {
  const [isDarkMode, setIsDarkMode] = useState(false);
  const [services, setServices] = useState<Service[] | null>(null);
  const [notifications, setNotifications] = useState<Notification[]>([]);

  const { user } = useUser();

  useEffect(() => {
    listServices()
      .then(setServices)
      .catch((err) => {
        console.error("listServices failed", err);
      });

    listNotifications()
      .then(setNotifications)
      .catch((err) => {
        console.error("listNotifications failed", err);
      });
  }, []);

  const onNotificationsViewed = useCallback(async (ids: string[]) => {
    const uniqueIds = [...new Set(ids)];
    if (uniqueIds.length === 0) return;

    setNotifications((prev) =>
      prev.map((notification) =>
        uniqueIds.includes(notification.id)
          ? { ...notification, read: true }
          : notification
      )
    );

    try {
      await Promise.all(uniqueIds.map((id) => markNotificationRead(id)));
    } catch (err) {
      console.error("markNotificationRead failed", err);

      setNotifications((prev) =>
        prev.map((notification) =>
          uniqueIds.includes(notification.id)
            ? { ...notification, read: false }
            : notification
        )
      );
    }
  }, []);

  const appSocket = useMemo(() => {
    return createWebSocketClient<AppSocketMessage>({
      path: "/ws",
      onOpen: () => {
        console.log("app websocket connected");
      },
      onClose: () => {
        console.log("app websocket disconnected");
      },
      onError: (event) => {
        console.error("app websocket error", event);
      },
      onMessage: (message) => {
        const nextService = mapService(message.service);

        setServices((prev) => {
          const current = prev ?? [];

          switch (message.type) {
            case "added": {
              const exists = current.some(
                (service) => service.id === nextService.id
              );

              if (exists) {
                return current.map((service) =>
                  service.id === nextService.id ? nextService : service
                );
              }

              return [nextService, ...current];
            }

            case "modified":
              return current.map((service) =>
                service.id === nextService.id ? nextService : service
              );

            case "deleted":
              return current.filter((service) => service.id !== nextService.id);

            default:
              return current;
          }
        });
      },
    });
  }, []);

  useEffect(() => {
    appSocket.connect();

    return () => {
      appSocket.disconnect();
    };
  }, [appSocket]);

  return (
    <div className={isDarkMode ? "app-shell app-shell--dark" : "app-shell app-shell--light"}>
      <Header
        isDarkMode={isDarkMode}
        onToggleTheme={() => setIsDarkMode((prev) => !prev)}
        user={user}
        onSignOut={() => signOut()}
        notifications={notifications}
        onNotificationsViewed={onNotificationsViewed}
      />
      <Content services={services} />
    </div>
  );
}
