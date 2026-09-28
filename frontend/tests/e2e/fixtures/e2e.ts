import { type Route, test as base, expect } from "@playwright/test";
import { getResponse, HttpHandler } from "msw";
import { handlers } from "../../../src/mocks/handlers";
import { resetStore } from "../../../src/mocks/store";

const httpHandlers = handlers.filter((h): h is HttpHandler => h instanceof HttpHandler);

/**
 * Serve a Playwright-intercepted request from the MSW handler set. This lets
 * the e2e suite reuse the stateful mock store (users, groups, roles, service
 * gates) without registering a service worker, which the suite blocks.
 */
async function fulfillFromMsw(route: Route) {
  const req = route.request();
  const body = req.postDataBuffer();
  const request = new Request(req.url(), {
    method: req.method(),
    headers: req.headers(),
    body: body && body.length > 0 ? new Uint8Array(body) : undefined,
  });
  const response = await getResponse(httpHandlers, request, {
    baseUrl: new URL(req.url()).origin,
  });
  if (!response) {
    await route.fulfill({ status: 404, body: "" });
    return;
  }
  await route.fulfill({
    status: response.status,
    headers: Object.fromEntries(response.headers.entries()),
    body: Buffer.from(await response.arrayBuffer()),
  });
}

export const test = base.extend<{
  mockApp: void;
}>({
  mockApp: [
    async ({ context }, use) => {
      resetStore();

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

      // Identity + admin endpoints come from the MSW handlers so the e2e
      // suite and the VITE_USE_MOCKS dev loop see the same data. The mocked
      // caller is a member of /admin; tests for non-admins override
      // caller-identity with a page-level route.
      await context.route(
        /^https?:\/\/[^/]+\/api\/v1\/(?:caller-identity|admin(?:\/|$))/,
        fulfillFromMsw,
      );

      await context.routeWebSocket(/^wss?:\/\/[^/]+\/api\/v1\/ws(?:\?.*)?$/, () => {});

      await use();
    },
    { auto: true },
  ],
});

export { expect };
