import { test as setup, expect } from "@playwright/test";

const authFile = ".playwright/auth/user.json";

setup("authenticate with Keycloak", async ({ page }, testInfo) => {
  setup.setTimeout(90_000);

  const username = process.env.E2E_USERNAME;
  const password = process.env.E2E_PASSWORD;

  if (!username || !password) {
    throw new Error("Missing E2E_USERNAME or E2E_PASSWORD");
  }

  await page.goto("/", { waitUntil: "commit" }).catch((error) => {
    const message = String(error);
    if (
      message.includes("ERR_ABORTED") ||
      message.includes("frame was detached")
    ) {
      return;
    }
    throw error;
  });

  const usernameField = page.getByLabel(/username|email/i);
  const passwordField = page.locator('input[name="password"]');

  // Wait until either the login form is visible, or the app is already visible.
  await Promise.race([
    expect(usernameField).toBeVisible({ timeout: 30_000 }),
    expect(page.locator("header")).toBeVisible({ timeout: 30_000 }),
  ]).catch(() => {
    throw new Error(`Neither login form nor app became visible. Current URL: ${page.url()}`);
  });

  // If we're on Keycloak, sign in.
  if (await usernameField.isVisible().catch(() => false)) {
    await usernameField.fill(username);
    await passwordField.fill(password);
    await page.getByRole("button", { name: /sign in|log in/i }).click();
  }

  // Handle the occasional 403 recovery page.
  const goBack = page.getByText(/go back/i);
  if (await goBack.isVisible().catch(() => false)) {
    await goBack.click();
  }

  // Wait until the authenticated app is actually rendered.
  await expect
    .poll(
      async () => {
        const candidates = [
          page.getByRole("button", { name: /notifications/i }),
          page.getByText("Pinned services"),
          page.getByRole("button", { name: /all services/i }),
          page.locator("header"),
          page.locator("main"),
        ];

        for (const locator of candidates) {
          if (await locator.first().isVisible().catch(() => false)) {
            return true;
          }
        }

        return false;
      },
      {
        timeout: 30_000,
        message: `Waiting for authenticated app. Current URL: ${page.url()}`,
      }
    )
    .toBe(true);

  await testInfo.attach("post-login-url", {
    body: page.url(),
    contentType: "text/plain",
  });

  await page.context().storageState({ path: authFile });
});
