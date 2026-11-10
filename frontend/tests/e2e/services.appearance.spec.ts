import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures/e2e";

type ThemeMode = "light" | "dark";

/** Opens the table view with the theme set through the app's own preference. */
async function openTable(page: Page, theme: ThemeMode) {
  await page.goto("/");
  await page.evaluate((mode) => window.localStorage.setItem("launchpad:themeMode", mode), theme);
  await page.goto("/");
  await page.getByRole("button", { name: "Table" }).click();
  return page.getByRole("table", { name: "Services" });
}

const backgroundOf = (locator: Locator) =>
  locator.evaluate((element) => getComputedStyle(element).backgroundColor);

test("dark table rows expose a visible hover surface", async ({ page }) => {
  const table = await openTable(page, "dark");
  const serviceRow = table.getByRole("row", { name: /JupyterHub/i });
  const beforeHover = await backgroundOf(serviceRow);

  await serviceRow.hover();
  await expect(serviceRow).not.toHaveCSS("background-color", beforeHover);
});

test("table header remains visually distinct from the table body in both themes", async ({
  page,
}) => {
  const pageBackground = {} as Record<ThemeMode, string>;

  for (const theme of ["light", "dark"] as const) {
    const table = await openTable(page, theme);
    const headerCell = table.getByRole("columnheader").first();

    pageBackground[theme] = await backgroundOf(page.locator("body"));
    expect(await backgroundOf(headerCell), theme).not.toBe(await backgroundOf(table));
  }

  // Guards against the dark pass silently rendering the light theme.
  expect(pageBackground.dark).not.toBe(pageBackground.light);
});

test("accordion sections do not add a divider between content groups", async ({ page }) => {
  await page.goto("/");

  const items = page.locator('[data-slot="accordion-item"]');
  await expect(items).toHaveCount(2);

  const dividerWidths = await items.evaluateAll((elements) =>
    elements.map((element) => Number.parseFloat(getComputedStyle(element).borderBottomWidth)),
  );
  expect(dividerWidths[0]).toBe(dividerWidths[1]);
});

test("pinned service card title is not underlined", async ({ page }) => {
  await page.goto("/");

  const pinnedServices = page.getByRole("region", { name: /^Pinned/ });
  const pinnedService = pinnedServices.getByRole("link", { name: /JupyterHub/i });
  const pinnedTitle = pinnedServices.locator('[data-slot="card-title"]', { hasText: "JupyterHub" });

  await expect(pinnedService).toHaveCSS("text-decoration-line", "none");
  await expect(pinnedTitle).toHaveCSS("text-decoration-line", "none");
  await pinnedService.hover();
  await expect(pinnedService).toHaveCSS("text-decoration-line", "none");
  await expect(pinnedTitle).toHaveCSS("text-decoration-line", "none");
});
