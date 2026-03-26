import { useCallback, useEffect, useMemo, useState } from "react";
import { listServices, type Service } from "../api/listServices";
import {
  listNotifications,
  markNotificationRead,
  type Notification,
} from "../api/notifications";
import { createWebSocketClient } from "../api/ws";
import { mapService } from "../api/mapServices";
import { deletePin, putPin } from "../api/pin";
import type { ServiceSocketMessage } from "../api/servicesSocket";
import type { NotificationSocketMessage } from "../api/notificationsSocket";

type AppSocketMessage = ServiceSocketMessage | NotificationSocketMessage;

const mockServices: Service[] = [
  {
    id: "svc-1",
    name: "JupyterHub",
    status: "Healthy",
    description: "Notebook platform",
    category: ["Data Science"],
    pinned: true,
    image: "",
    url: "https://example.com/jupyterhub",
  },
  {
    id: "svc-2",
    name: "Grafana",
    status: "Unhealthy",
    description: "Metrics dashboards",
    category: ["Monitoring"],
    pinned: false,
    image: "",
    url: "https://example.com/grafana",
  },
  {
    id: "svc-3",
    name: "Admin Panel",
    status: "Unknown",
    description: "Administrative tools",
    category: ["Platform"],
    pinned: false,
    image: "",
    url: "https://example.com/admin",
  },
];

const mockNotifications: Notification[] = [
  {
    id: "notif-1",
    image: "",
    title: "JupyterHub is back online!",
    message: "JupyterHub is back online and ready to use.",
    read: false,
    createdAt: new Date().toISOString(),
  },
  {
    id: "notif-2",
    image: "",
    title: "Scheduled maintenance planned",
    message: "Maintenance will occur on the first Saturday of each month.",
    read: true,
    createdAt: new Date().toISOString(),
  },
];

export function useLaunchpadData(user: unknown) {
  const shouldBypassAuth = import.meta.env.VITE_BYPASS_AUTH === "true";

  const [services, setServices] = useState<Service[]>(() =>
    shouldBypassAuth ? mockServices : []
  );
  const [notifications, setNotifications] = useState<Notification[]>(() =>
    shouldBypassAuth ? mockNotifications : []
  );

  useEffect(() => {
    if (shouldBypassAuth) return;

    listServices().then(setServices).catch(console.error);
    listNotifications().then(setNotifications).catch(console.error);
  }, [user, shouldBypassAuth]);

  const onNotificationsViewed = useCallback(
    async (ids: string[]) => {
      const uniqueIds = [...new Set(ids)];
      if (uniqueIds.length === 0) return;

      setNotifications((prev) =>
        prev.map((notification) =>
          uniqueIds.includes(notification.id)
            ? { ...notification, read: true }
            : notification
        )
      );

      if (shouldBypassAuth) return;

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
    },
    [shouldBypassAuth]
  );

  const onTogglePin = useCallback(
    async (serviceId: string, nextPinned: boolean) => {
      let previousPinned: boolean | undefined;

      setServices((prev) =>
        prev.map((service) => {
          if (service.id === serviceId) {
            previousPinned = service.pinned;
            return { ...service, pinned: nextPinned };
          }
          return service;
        })
      );

      if (shouldBypassAuth) return;

      try {
        if (nextPinned) {
          await putPin(serviceId);
        } else {
          await deletePin(serviceId);
        }
      } catch (err) {
        console.error("toggle pin failed", err);

        if (previousPinned === undefined) return;
        const rollbackPinned = previousPinned;

        setServices((prev) =>
          prev.map((service) =>
            service.id === serviceId
              ? { ...service, pinned: rollbackPinned }
              : service
          )
        );
      }
    },
    [shouldBypassAuth]
  );

  const appSocket = useMemo(() => {
    if (shouldBypassAuth) return null;

    return createWebSocketClient<AppSocketMessage>({
      path: "/ws",
      onOpen: () => console.log("app websocket connected"),
      onClose: () => console.log("app websocket disconnected"),
      onError: (event) => console.error("app websocket error", event),
      onMessage: (message) => {
        if (message.type === "notification.created") {
          const nextNotification: Notification = {
            id: message.notification.id,
            title: message.notification.title,
            message: message.notification.message,
            createdAt: message.notification.createdAt,
            image: message.notification.image ?? "",
            read: message.notification.read ?? false,
          };

          setNotifications((prev) => {
            const exists = prev.some((n) => n.id === nextNotification.id);
            return exists ? prev : [nextNotification, ...prev];
          });

          return;
        }

        const nextService = mapService(message.service);

        setServices((prev) => {
          switch (message.type) {
            case "added": {
              const exists = prev.some(
                (service) => service.id === nextService.id
              );

              if (exists) {
                return prev.map((service) =>
                  service.id === nextService.id
                    ? { ...nextService, pinned: service.pinned }
                    : service
                );
              }

              return [nextService, ...prev];
            }

            case "modified":
              return prev.map((service) =>
                service.id === nextService.id
                  ? { ...nextService, pinned: service.pinned }
                  : service
              );

            case "deleted":
              return prev.filter((service) => service.id !== nextService.id);

            default:
              return prev;
          }
        });
      },
    });
  }, [shouldBypassAuth]);

  useEffect(() => {
    if (!appSocket) return;

    appSocket.connect();
    return () => appSocket.disconnect();
  }, [appSocket]);

  return {
    services,
    notifications,
    onNotificationsViewed,
    onTogglePin,
  };
}
