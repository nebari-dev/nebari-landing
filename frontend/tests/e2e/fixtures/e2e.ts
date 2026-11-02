import { test as base, expect } from "@playwright/test";
import { type MockService, richerServices } from "./services";

export const SERVICES_URL = /^https?:\/\/[^/]+\/api\/v1\/services\/?(?:\?.*)?$/;

export const test = base.extend<{
  /** Services served by the mocked API; override with `test.use({ services: [list, { scope: "test" }] })`. */
  services: MockService[];
  mockApp: void;
}>({
  services: [richerServices, { option: true }],
  mockApp: [
    async ({ context, services }, use) => {
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

      await context.route(SERVICES_URL, async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(services),
        });
      });

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
