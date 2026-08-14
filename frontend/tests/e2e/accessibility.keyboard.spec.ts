import { expect, test } from "./fixtures/e2e";

test("header controls are reachable by keyboard", async ({ page }) => {
  const notificationRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.startsWith("/api/v1/notifications")) {
      notificationRequests.push(request.url());
    }
  });

  await page.goto("/");

  await expect(page.getByRole("button", { name: /notifications/i })).toHaveCount(0);

  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: /go to homepage/i })).toBeFocused();

  // Notifications are hidden, so the account menu is the next focusable
  // header control after the logo.
  await page.keyboard.press("Tab");
  await expect(page.getByRole("button", { name: /account menu/i })).toBeFocused();
  expect(notificationRequests).toEqual([]);
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

  const tableContainer = allServices.locator('[data-slot="table-container"]');
  await page.keyboard.press("Tab");
  await expect(tableContainer).toBeFocused();

  await page.keyboard.press("Tab");
  await expect(tableContainer.getByRole("link", { name: /JupyterHub/i })).toBeFocused();
});

test("accordion trigger can be toggled with keyboard", async ({ page }) => {
  await page.goto("/");

  const pinnedTrigger = page.getByRole("button", { name: /Pinned services/i });

  await expect(pinnedTrigger).toBeVisible();

  await pinnedTrigger.focus();
  await expect(pinnedTrigger).toBeFocused();
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "true");

  await page.keyboard.press("Enter");
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "false");

  await page.keyboard.press(" ");
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "true");
});

test("header and accordion controls expose visible focus rings", async ({ page }) => {
  await page.goto("/");

  const readFocusShadow = (element: HTMLElement) => getComputedStyle(element).boxShadow;
  const accountMenu = page.getByRole("button", { name: /account menu/i });
  const pinnedTrigger = page.getByRole("button", { name: /Pinned services/i });

  await accountMenu.focus();
  const buttonFocusShadow = await accountMenu.evaluate(readFocusShadow);
  expect(buttonFocusShadow).not.toBe("none");

  await pinnedTrigger.focus();
  expect(await pinnedTrigger.evaluate(readFocusShadow)).not.toBe("none");
});

test("service card exposes a visible focus indicator", async ({ page }) => {
  await page.goto("/");

  const serviceCard = page.getByRole("link", { name: /JupyterHub/i }).first();
  await serviceCard.focus();
  await expect(serviceCard).toBeFocused();
  const cardSurface = serviceCard.locator('[data-slot="card"]');
  expect(await cardSurface.evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe(
    "none",
  );
});
