// MSW WebSocket interceptor for /api/v1/ws.
//
// On connection we send a "modified" service event so the UI demonstrates
// incoming realtime activity.

import { ws } from "msw";
import { store } from "./store";

const api = ws.link("ws://*/api/v1/ws");

export const wsHandlers = [
  api.addEventListener("connection", ({ client }) => {
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
