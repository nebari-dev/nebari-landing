// MSW WebSocket interceptor for /api/v1/ws.
//
// The SPA opens a single socket carrying both ServiceSocketMessage and
// NotificationSocketMessage frames (see hooks/useLaunchpadData.ts). On
// connection we send one mocked notification and one "modified" service
// event so the UI demonstrates incoming realtime activity.

import { ws } from "msw";
import { store } from "./store";

const api = ws.link("ws://*/api/v1/ws");

export const wsHandlers = [
  api.addEventListener("connection", ({ client }) => {
    // Push one mocked notification a moment after the socket opens.
    setTimeout(() => {
      const notification = {
        id: `ntf-mock-${Date.now()}`,
        image: "",
        title: "Realtime mock event",
        message: "This frame was sent by MSW's WebSocket interceptor.",
        read: false,
        createdAt: new Date().toISOString(),
      };
      store.notifications.unshift(notification);
      client.send(JSON.stringify({ type: "notification.created", notification }));
    }, 1500);

    // Push a "modified" service event so the launchpad reflects realtime
    // service updates without requiring a refresh.
    setTimeout(() => {
      const svc = store.services[0];
      if (!svc) return;
      client.send(
        JSON.stringify({
          type: "modified",
          service: {
            uid: svc.id,
            name: svc.name,
            namespace: "default",
            displayName: svc.name,
            description: svc.description,
            url: svc.url,
            icon: svc.image,
            category: svc.category[0] ?? "",
            priority: 0,
            visibility: "public",
            health: {
              status: svc.status,
              lastCheck: new Date().toISOString(),
            },
          },
        }),
      );
    }, 3000);
  }),
];
