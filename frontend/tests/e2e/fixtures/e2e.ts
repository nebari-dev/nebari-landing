import { test as base, expect } from "@playwright/test";

export const test = base.extend<{
  mockApp: void;
}>({
  mockApp: [
    async ({ context }, use) => {
      await context.addInitScript(() => {
        window.__PW_E2E_AUTH__ = {
          authenticated: true,
          token: "mock-token",
          idTokenParsed: {
            name: "Test User",
            email: "test.user@example.com",
            preferred_username: "test.user",
            sub: "e2e-user",
          },
        };
      });

      await context.route(
        /^https?:\/\/[^/]+\/api\/v1\/services\/?(?:\?.*)?$/,
        async (route) => {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify([
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
            ]),
          });
        },
      );

      await context.route(
        /^https?:\/\/[^/]+\/api\/v1\/pins\/[^/?]+(?:\?.*)?$/,
        async (route) => {
          if (route.request().method() === "GET") {
            const serviceId = new URL(route.request().url()).pathname.split("/").at(-1);
            await route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({ id: serviceId }),
            });
            return;
          }

          await route.fulfill({ status: 204, body: "" });
        },
      );

      await context.route(
        /^https?:\/\/[^/]+\/api\/v1\/ws-ticket\/?(?:\?.*)?$/,
        async (route) => {
          await route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ ticket: "mock-ws-ticket" }),
          });
        },
      );

      await context.routeWebSocket(/^wss?:\/\/[^/]+\/api\/v1\/ws(?:\?.*)?$/, () => {});

      await use();
    },
    { auto: true },
  ],
});

export { expect };
