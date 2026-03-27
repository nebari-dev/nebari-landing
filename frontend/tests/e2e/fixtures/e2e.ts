import { test as base, expect } from "@playwright/test";

export const test = base.extend<{
  mockApp: void;
}>({
  mockApp: [
    async ({ context, page }, use) => {
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

      page.on("request", (req) => {
        if (req.url().includes("/api/")) {
          console.log("REQ", req.method(), req.url());
        }
      });

      await context.route(/\/api\/.*services(?:\/)?(?:\?.*)?$/, async (route) => {
        console.log("MOCK services", route.request().url());
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
      });

      await context.route(/\/api\/.*notifications(?:\/)?(?:\?.*)?$/, async (route) => {
        console.log("MOCK notifications", route.request().url());
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([
            {
              id: "notif-1",
              image: "",
              title: "JupyterHub is back online",
              message: "Ready to use.",
              read: false,
              createdAt: new Date().toISOString(),
            },
          ]),
        });
      });

      await context.route(/\/api\/notifications\/.+/, async (route) => {
        await route.fulfill({ status: 204, body: "" });
      });

      await context.route(/\/api\/pin\/.+/, async (route) => {
        await route.fulfill({ status: 204, body: "" });
      });

      await use();
    },
    { auto: true },
  ],
});

export { expect };
export {};
