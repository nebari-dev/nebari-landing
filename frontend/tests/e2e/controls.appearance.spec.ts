import { expect, test } from "./fixtures/e2e";

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
