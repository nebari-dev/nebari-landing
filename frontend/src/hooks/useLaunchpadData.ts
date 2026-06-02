import { useCallback, useEffect, useMemo, useState } from "react";
import { apiFetch } from "../api/client";
import { listServices, type Service } from "../api/listServices";
import { mapService } from "../api/mapServices";
import { listNotifications, markNotificationRead, type Notification } from "../api/notifications";
import type { NotificationSocketMessage } from "../api/notificationsSocket";
import { deletePin, putPin } from "../api/pin";
import type { ServiceSocketMessage } from "../api/servicesSocket";
import { createWebSocketClient } from "../api/ws";

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
        uniqueIds.includes(notification.id) ? { ...notification, read: true } : notification,
      ),
    );

    try {
      await Promise.all(uniqueIds.map((id) => markNotificationRead(id)));
    } catch (err) {
      console.error("markNotificationRead failed", err);
      setNotifications((prev) =>
        prev.map((notification) =>
          uniqueIds.includes(notification.id) ? { ...notification, read: false } : notification,
        ),
      );
    }
  }, []);

  const onTogglePin = useCallback(async (serviceId: string, nextPinned: boolean) => {
    let previousPinned: boolean | undefined;

    setServices((prev) =>
      prev.map((service) => {
        if (service.id === serviceId) {
          previousPinned = service.pinned;
          return { ...service, pinned: nextPinned };
        }
        return service;
      }),
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
          service.id === serviceId ? { ...service, pinned: previousPinned! } : service,
        ),
      );
    }
  }, []);

  // The useMemo dependency on `user` rebuilds this WS client whenever the
  // auth state flips. On a fresh page load that typically fires twice: once
  // on initial mount with user=null (the ticket POST may 401, the silent
  // fallback below kicks in), and once after keycloak-js resolves and user
  // becomes the parsed claims. Two /ws-ticket POSTs per page load is
  // expected. Steady state is one redeemed ticket per WS lifetime — a
  // sustained drumbeat of fresh POSTs without disconnect events would
  // indicate a real reconnect storm.
  const appSocket = useMemo(() => {
    const isAuthenticated = Boolean(user);

    return createWebSocketClient<AppSocketMessage>({
      path: "/ws",
      // Fetch a fresh single-use ticket before each connect (and each reconnect).
      // Browsers cannot send Authorization headers on WebSocket upgrade requests,
      // so the webapi accepts a short-lived ticket as an alternative.
      // When the exchange fails (e.g. auth disabled in dev), connect without a ticket.
      getQueryParams: async () => {
        try {
          const resp = await apiFetch("/ws-ticket", { method: "POST" });
          if (resp.ok) {
            const data = (await resp.json()) as { ticket: string };
            return { ticket: data.ticket };
          }
        } catch {
          // Ticket exchange failed — proceed without one (auth-disabled dev mode).
        }
        return {};
      },
      onOpen: () => console.log("app websocket connected", { authenticated: isAuthenticated }),
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
                    : service,
                );
              }

              return [nextService, ...prev];
            }

            case "modified":
              if (prev.some((service) => service.id === nextService.id)) {
                return prev.map((service) =>
                  service.id === nextService.id
                    ? { ...nextService, pinned: service.pinned }
                    : service,
                );
              }

              // A service can be re-enabled and come back as "modified" after a
              // prior "deleted" event. Upsert it so the UI reflects re-additions
              // without requiring a page refresh.
              return [nextService, ...prev];

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
