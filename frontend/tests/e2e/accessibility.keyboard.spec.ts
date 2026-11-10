import type { Locator } from "@playwright/test";
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

test("view toggle switches views from the keyboard", async ({ page }) => {
  await page.goto("/");

  const gridView = page.getByRole("button", { name: "Grid" });
  const tableView = page.getByRole("button", { name: "Table" });

  await gridView.focus();
  await page.keyboard.press("Tab");
  await expect(tableView).toBeFocused();
  await expect(gridView).toHaveAttribute("aria-pressed", "true");
  await expect(tableView).toHaveAttribute("aria-pressed", "false");

  await page.keyboard.press("Enter");
  await expect(tableView).toHaveAttribute("aria-pressed", "true");

  const tableContainer = page.locator('[data-slot="table-container"]');
  await expect(tableContainer).toBeVisible();
  await expect(tableContainer).not.toHaveAttribute("tabindex");

  for (const header of ["Service", "Category", "Status"]) {
    await page.keyboard.press("Tab");
    await expect(tableContainer.getByRole("button", { name: header, exact: true })).toBeFocused();
  }

  await page.keyboard.press("Tab");
  await expect(tableContainer.getByRole("button", { name: "Unpin service" })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(tableContainer.getByRole("link", { name: /JupyterHub/i })).toBeFocused();
});

test("accordion trigger can be toggled with keyboard", async ({ page }) => {
  await page.goto("/");

  const pinnedTrigger = page.getByRole("button", { name: /^Pinned/ });

  await expect(pinnedTrigger).toBeVisible();

  await pinnedTrigger.focus();
  await expect(pinnedTrigger).toBeFocused();
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "true");

  await page.keyboard.press("Enter");
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "false");

  await page.keyboard.press(" ");
  await expect(pinnedTrigger).toHaveAttribute("aria-expanded", "true");
});

test("service cards are reachable from the keyboard", async ({ page }) => {
  await page.goto("/");

  const pinnedTrigger = page.getByRole("button", { name: /^Pinned/ });
  const pinnedSection = page.getByRole("region", { name: /^Pinned/ });

  await pinnedTrigger.focus();
  await page.keyboard.press("Tab");
  await expect(
    pinnedSection.getByRole("link", { name: "JupyterHub (opens in a new tab)" }),
  ).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(pinnedSection.getByRole("button", { name: "Unpin service" })).toBeFocused();
});

/** Outline and shadow together, so any focus ring changes it whatever its color or mechanism. */
const readFocusIndicator = (locator: Locator) =>
  locator.evaluate((element) => {
    const style = getComputedStyle(element);
    return [style.outlineStyle, style.outlineWidth, style.boxShadow].join(" ");
  });

test("header and accordion controls show a focus indicator", async ({ page }) => {
  await page.goto("/");

  for (const control of [
    page.getByRole("button", { name: /account menu/i }),
    page.getByRole("button", { name: /^Pinned/ }),
  ]) {
    const unfocused = await readFocusIndicator(control);
    await control.focus();
    expect(await readFocusIndicator(control)).not.toBe(unfocused);
  }
});

test("service card shows a focus indicator", async ({ page }) => {
  await page.goto("/");

  // The card's link is a full-size overlay; its focus ring is drawn on the card beside it.
  const pinnedSection = page.getByRole("region", { name: /^Pinned/ });
  const serviceLink = pinnedSection.getByRole("link", { name: /JupyterHub/i });
  const cardSurface = pinnedSection.locator('[data-slot="card"]', { hasText: "JupyterHub" });

  const unfocused = await readFocusIndicator(cardSurface);
  await serviceLink.focus();
  await expect(serviceLink).toBeFocused();
  expect(await readFocusIndicator(cardSurface)).not.toBe(unfocused);
});
