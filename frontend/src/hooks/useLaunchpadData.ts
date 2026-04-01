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

export function useLaunchpadData(user: unknown) {
  const [services, setServices] = useState<Service[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);

  useEffect(() => {
    listServices().then(setServices).catch(console.error);
    listNotifications().then(setNotifications).catch(console.error);
  }, [user]);

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

      try {
        if (nextPinned) {
          await putPin(serviceId);
        } else {
          await deletePin(serviceId);
        }
      } catch (err) {
        console.error("toggle pin failed", err);

        if (previousPinned === undefined) return;

        setServices((prev) =>
          prev.map((service) =>
            service.id === serviceId
              ? { ...service, pinned: previousPinned! }
              : service
          )
        );
      }
    },
    []
  );

  const appSocket = useMemo(() => {
    const isAuthenticated = Boolean(user);

    return createWebSocketClient<AppSocketMessage>({
      path: "/ws",
      onOpen: () =>
        console.log("app websocket connected", { authenticated: isAuthenticated }),
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
              const exists = prev.some((service) => service.id === nextService.id);

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
  }, [user]);

  useEffect(() => {
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
