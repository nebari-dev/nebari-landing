import { expect, test } from "./fixtures/e2e";

test("header controls are reachable by keyboard", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("button", { name: /notifications/i })).toBeVisible();

  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: /go to homepage/i })).toBeFocused();

  // The theme toggle now lives in the profile menu, so notifications is the
  // next focusable header control after the logo.
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: /notifications/i })).toBeFocused();
});

test("Ctrl+K focuses the search input", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByPlaceholder("Search")).toBeVisible();

  await page.keyboard.press("ControlOrMeta+K");
  await expect(page.getByPlaceholder("Search")).toBeFocused();
});

test("view tabs use arrow-key focus and manual activation", async ({ page }) => {
  await page.goto("/");

  const allServices = page.getByRole("region", { name: /All services/i });
  const gridView = allServices.getByRole("tab", { name: /Grid View/i });
  const listView = allServices.getByRole("tab", { name: /List View/i });

  await gridView.focus();
  await expect(gridView).toBeFocused();

  await page.keyboard.press("ArrowRight");
  await expect(listView).toBeFocused();
  await expect(gridView).toHaveAttribute("aria-selected", "true");
  await expect(listView).toHaveAttribute("aria-selected", "false");

  await page.keyboard.press("Enter");
  await expect(listView).toHaveAttribute("aria-selected", "true");

  await page.keyboard.press("Tab");
  await expect(allServices.getByRole("link", { name: /JupyterHub/i })).toBeFocused();
});

test("dark table rows expose a visible hover surface", async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => document.documentElement.classList.add("dark"));

  const allServices = page.getByRole("region", { name: /All services/i });
  await allServices.getByRole("tab", { name: /List View/i }).click();
  const serviceRow = allServices.getByRole("link", { name: /JupyterHub/i });
  const beforeHover = await serviceRow.evaluate(
    (element) => getComputedStyle(element).backgroundColor,
  );

  await serviceRow.hover();
  await expect(serviceRow).not.toHaveCSS("background-color", beforeHover);
});

test("accordion trigger can be toggled with keyboard", async ({ page }) => {
  await page.goto("/");

  const pinnedTrigger = page.getByRole("button", { name: /Pinned services/i });

  await expect(pinnedTrigger).toBeVisible();

  await pinnedTrigger.focus();
  await expect(pinnedTrigger).toBeFocused();
  await expect(pinnedTrigger).toHaveCSS("border-radius", "8px");

  const notificationButton = page.getByRole("button", { name: /notifications/i });
  await notificationButton.focus();
  const buttonFocusStyle = await notificationButton.evaluate((element) => {
    const style = getComputedStyle(element);
    return {
      color: style.getPropertyValue("--tw-ring-color"),
      offsetWidth: style.getPropertyValue("--tw-ring-offset-width"),
    };
  });
  await pinnedTrigger.focus();
  expect(
    await pinnedTrigger.evaluate((element) => {
      const style = getComputedStyle(element);
      return {
        color: style.getPropertyValue("--tw-ring-color"),
        offsetWidth: style.getPropertyValue("--tw-ring-offset-width"),
      };
    }),
  ).toEqual(buttonFocusStyle);
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "true");

  await page.keyboard.press("Enter");
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "false");

  await page.keyboard.press(" ");
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "true");
});

test("service card focus ring overlays the card border", async ({ page }) => {
  await page.goto("/");

  const serviceCard = page.getByRole("link", { name: /JupyterHub/i }).first();
  await serviceCard.focus();
  await expect(serviceCard).toBeFocused();
  await expect(serviceCard).toHaveCSS("border-radius", "8px");
  const cardSurface = serviceCard.locator('[data-slot="card"]');
  expect(await cardSurface.evaluate((element) => getComputedStyle(element).boxShadow)).toContain(
    "inset",
  );
});
