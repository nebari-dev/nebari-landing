import { expect, test } from "./fixtures/e2e";

test("notifications dropdown opens and closes with keyboard", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /notifications/i });

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
