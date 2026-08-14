import type { Locator } from "@playwright/test";
import { expect, test } from "./fixtures/e2e";

async function getBox(locator: Locator) {
  const box = await locator.boundingBox();
  if (!box) throw new Error("Expected locator to have a bounding box");
  return box;
}

test("active selectors are visually distinct from inactive selectors", async ({ page }) => {
  await page.goto("/");

  const activeView = page.getByRole("tab", { name: /Grid View/i });
  const inactiveView = page.getByRole("tab", { name: /List View/i });

  await page.getByRole("button", { name: /account menu/i }).click();
  const activeTheme = page.getByRole("menuitemradio", { name: /system theme/i });
  const inactiveTheme = page.getByRole("menuitemradio", { name: /light mode/i });

  const readSurface = (element: HTMLElement) => {
    const styles = getComputedStyle(element);
    return {
      background: styles.backgroundColor,
      border: styles.borderTopColor,
      foreground: styles.color,
    };
  };

  for (const [active, inactive] of [
    [activeView, inactiveView],
    [activeTheme, inactiveTheme],
  ] as const) {
    const [activeSurface, inactiveSurface] = await Promise.all([
      active.evaluate(readSurface),
      inactive.evaluate(readSurface),
    ]);

    expect(activeSurface.background).not.toBe(inactiveSurface.background);
    expect(activeSurface.border).not.toBe(inactiveSurface.border);
    expect(activeSurface.foreground).not.toBe(inactiveSurface.foreground);
  }
});

test("notifications menu border contrasts with its surface in both themes", async ({ page }) => {
  const expectBorderContrast = async () => {
    const styles = await page.getByRole("menu").evaluate((element) => {
      const computed = getComputedStyle(element);
      return {
        background: computed.backgroundColor,
        border: computed.borderTopColor,
        borderWidth: Number.parseFloat(computed.borderTopWidth),
      };
    });

    expect(styles.borderWidth).toBeGreaterThan(0);
    expect(styles.border).not.toBe(styles.background);
  };

  await page.goto("/");
  await page.getByRole("button", { name: /notifications/i }).click();
  await expectBorderContrast();

  await page.keyboard.press("Escape");
  await page.evaluate(() => window.localStorage.setItem("launchpad:themeMode", "dark"));
  await page.reload();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await page.getByRole("button", { name: /notifications/i }).click();
  await expectBorderContrast();
});

test("only non-final notification rows expose separators", async ({ page }) => {
  await page.route(
    /^https?:\/\/[^/]+\/api\/v1\/notifications\/?(?:\?.*)?$/,
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "notif-1",
            title: "JupyterHub is back online",
            message: "Ready to use.",
            read: false,
            createdAt: new Date().toISOString(),
          },
          {
            id: "notif-2",
            title: "Grafana maintenance complete",
            message: "Dashboards are available.",
            read: true,
            createdAt: new Date().toISOString(),
          },
        ]),
      });
    },
  );

  await page.goto("/");
  await page.getByRole("button", { name: /notifications/i }).click();

  const rows = page.getByRole("menuitem");
  await expect(rows).toHaveCount(2);
  const separatorWidths = await rows.evaluateAll((elements) =>
    elements.map((element) => Number.parseFloat(getComputedStyle(element).borderBottomWidth)),
  );
  expect(separatorWidths[0]).toBeGreaterThan(separatorWidths[1]);
});

test("sign out action is visually distinguished from neutral menu actions", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: /account menu/i }).click();

  const signOut = page.getByRole("menuitem", { name: /sign out/i });
  const themeOption = page.getByRole("menuitemradio", { name: /system theme/i });
  const [signOutColor, themeOptionColor] = await Promise.all([
    signOut.evaluate((element) => getComputedStyle(element).color),
    themeOption.evaluate((element) => getComputedStyle(element).color),
  ]);

  expect(signOutColor).not.toBe(themeOptionColor);
  await expect(signOut.locator("svg")).toBeVisible();
});

test("header actions remain separated while focus indicators are visible", async ({ page }) => {
  await page.goto("/");

  const notificationButton = page.getByRole("button", { name: /notifications/i });
  const accountMenu = page.getByRole("button", { name: /account menu/i });
  await accountMenu.focus();
  await expect(accountMenu).toBeFocused();

  const [notificationBox, accountBox] = await Promise.all([
    getBox(notificationButton),
    getBox(accountMenu),
  ]);
  expect(accountBox.x).toBeGreaterThanOrEqual(notificationBox.x + notificationBox.width);
});

test("notification badge remains subordinate to its trigger", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /notifications/i });
  const badge = trigger.locator("span");
  const [triggerBox, badgeBox] = await Promise.all([getBox(trigger), getBox(badge)]);

  expect(badgeBox.height).toBeLessThan(triggerBox.height);
  expect(badgeBox.width).toBeGreaterThanOrEqual(badgeBox.height);
});
