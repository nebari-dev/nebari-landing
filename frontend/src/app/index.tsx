import { useEffect, useMemo, useState } from "react";
import Header from "../components/header";
import Content from "../components/content";
import { signOut } from "../auth/keycloak";
import { useUser } from "../auth/user";

import { listServices, type Service } from "../api/listServices";
import { listNotifications, type Notification } from "../api/notifications";
import { createWebSocketClient } from "../api/ws";

type AppSocketMessage =
  | {
      type: "service.created";
      service: Service;
    }
  | {
      type: "service.updated";
      service: Service;
    }
  | {
      type: "service.deleted";
      id: string;
    }
  | {
      type: "notification.created";
      notification: Notification;
    }
  | {
      type: "notification.updated";
      notification: Notification;
    }
  | {
      type: "notification.read";
      id: string;
    };

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
        switch (message.type) {
          case "service.created": {
            setServices((prev) => {
              const current = prev ?? [];
              const exists = current.some(
                (service) => service.id === message.service.id
              );

              if (exists) {
                return current.map((service) =>
                  service.id === message.service.id ? message.service : service
                );
              }

              return [message.service, ...current];
            });
            break;
          }

          case "service.updated": {
            setServices((prev) => {
              const current = prev ?? [];
              return current.map((service) =>
                service.id === message.service.id ? message.service : service
              );
            });
            break;
          }

          case "service.deleted": {
            setServices((prev) => {
              const current = prev ?? [];
              return current.filter((service) => service.id !== message.id);
            });
            break;
          }

          case "notification.created": {
            setNotifications((prev) => {
              const exists = prev.some(
                (notification) => notification.id === message.notification.id
              );

              if (exists) {
                return prev.map((notification) =>
                  notification.id === message.notification.id
                    ? message.notification
                    : notification
                );
              }

              return [message.notification, ...prev];
            });
            break;
          }

          case "notification.updated": {
            setNotifications((prev) =>
              prev.map((notification) =>
                notification.id === message.notification.id
                  ? message.notification
                  : notification
              )
            );
            break;
          }

          case "notification.read": {
            setNotifications((prev) =>
              prev.map((notification) =>
                notification.id === message.id
                  ? { ...notification, read: true }
                  : notification
              )
            );
            break;
          }
        }
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
      />
      <Content services={services} />
    </div>
  );
}
