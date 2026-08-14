import { expect, test } from "./fixtures/e2e";

test("notifications dropdown opens and closes with keyboard", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /notifications/i });

  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(255, 255, 255)");
  await expect(page.locator("header")).toHaveCSS("background-color", "rgb(248, 248, 248)");
  await expect(page.locator("header")).toHaveCSS("border-bottom-color", "rgb(183, 183, 187)");
  expect((await page.getByRole("link", { name: "Go to homepage" }).boundingBox())?.x).toBe(16);
  await trigger.hover();
  await expect(trigger).toHaveCSS("background-color", "rgb(217, 217, 220)");

  await trigger.focus();
  await expect(trigger).toBeFocused();

  await page.keyboard.press("Enter");

  await expect(page.getByRole("menu")).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).not.toBeVisible();
  await expect(trigger).toBeFocused();
});

test("account dropdown opens and closes with keyboard", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /account menu/i });
  await trigger.focus();
  await page.keyboard.press("Enter");

  await expect(page.getByRole("menu")).toBeVisible();
  await expect(page.getByRole("group", { name: "Theme" })).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).not.toBeVisible();
  await expect(trigger).toBeFocused();
});

test("notifications dropdown preserves page scrolling", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 500 });
  await page.goto("/");
  await page.evaluate(() => {
    document.body.style.minHeight = "200vh";
  });

  await page.getByRole("button", { name: /notifications/i }).click();
  await expect(page.getByRole("menu")).toBeVisible();
  await expect(page.locator("body")).not.toHaveAttribute("data-scroll-locked");

  const initialScrollY = await page.evaluate(() => window.scrollY);
  await page.mouse.move(100, 400);
  await page.mouse.wheel(0, 500);

  await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(initialScrollY);
});

test("notification dropdown scrolls when the list exceeds the viewport", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 500 });
  await page.route(
    /^https?:\/\/[^/]+\/api\/v1\/notifications\/?(?:\?.*)?$/,
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(
          Array.from({ length: 12 }, (_, index) => ({
            id: `scroll-notification-${index + 1}`,
            title: `Mock notification ${index + 1}`,
            message: "Notification content used to verify scrolling.",
            read: index % 2 === 0,
            createdAt: new Date(Date.now() - index * 60_000).toISOString(),
          })),
        ),
      });
    },
  );

  await page.goto("/");
  await page.getByRole("button", { name: /notifications/i }).click();

  const menu = page.getByRole("menu");
  await expect(menu).toBeVisible();
  expect(await menu.evaluate((element) => element.scrollHeight > element.clientHeight)).toBe(true);

  await menu.evaluate((element) => element.scrollTo({ top: element.scrollHeight }));
  await expect.poll(() => menu.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
});

test("sign out action uses body-small typography", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: /account menu/i }).click();

  const signOut = page.getByRole("menuitem", { name: /sign out/i });
  await expect(signOut).toHaveCSS("color", "rgb(210, 22, 28)");
  await expect(signOut).toHaveCSS("font-family", /Inter Variable/);
  await expect(signOut).toHaveCSS("font-size", "14px");
  await expect(signOut).toHaveCSS("font-style", "normal");
  await expect(signOut).toHaveCSS("font-weight", "400");
  await expect(signOut).toHaveCSS("line-height", "20px");

  const icon = signOut.locator("svg");
  await expect(icon).toHaveCSS("width", "16px");
  await expect(icon).toHaveCSS("height", "16px");
  await expect(icon).toHaveCSS("flex-shrink", "0");
});

test("header actions leave room for the profile focus ring", async ({ page }) => {
  await page.goto("/");

  const actions = page.locator('[data-slot="menu-bar-actions"]');
  const accountMenu = page.getByRole("button", { name: /account menu/i });

  await expect(actions).toHaveCSS("column-gap", "8px");
  await expect(accountMenu).toHaveCSS("padding-left", "10px");
  await expect(accountMenu).toHaveCSS("padding-right", "10px");
  await expect(accountMenu).toHaveCSS("padding-top", "4px");
  await expect(accountMenu).toHaveCSS("padding-bottom", "4px");
});

test("compact icon button focus ring has no offset gap", async ({ page }) => {
  await page.goto("/");

  const notificationButton = page.getByRole("button", { name: /notifications/i });
  await notificationButton.focus();

  const focusShadow = await notificationButton.evaluate(
    (element) => getComputedStyle(element).boxShadow,
  );
  expect(focusShadow).not.toContain("0px 0px 0px 4px");
});

test("notification badge uses the compact size", async ({ page }) => {
  await page.goto("/");

  const badge = page.getByRole("button", { name: /notifications/i }).locator("span");
  await expect(badge).toHaveCSS("height", "16px");
  await expect(badge).toHaveCSS("min-width", "16px");
  await expect(badge).toHaveCSS("align-items", "center");
  await expect(badge).toHaveCSS("justify-content", "center");
  await expect(badge).toHaveCSS("line-height", "9px");
});
