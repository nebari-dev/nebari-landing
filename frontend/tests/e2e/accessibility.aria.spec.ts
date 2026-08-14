import { expect, test } from "./fixtures/e2e";

test("header accessibility tree stays stable", async ({ page }) => {
  await page.goto("/");

  const header = page.locator("header");
  await expect(header).toBeVisible();

  // Notifications are intentionally hidden, leaving the account-menu trigger
  // as the only header action next to the logo.
  await expect(header).toMatchAriaSnapshot(`
    - link "Go to homepage"
    - button "Account menu"
  `);
});

test("services controls accessibility tree stays stable", async ({ page }) => {
  await page.goto("/");

  const allServicesRegion = page.getByRole("region", { name: /All services/i });
  await expect(allServicesRegion).toBeVisible();

  await expect(allServicesRegion.getByRole("textbox", { name: "Search services" })).toBeVisible();
  await expect(allServicesRegion.getByRole("tablist")).toMatchAriaSnapshot(`
    - tablist:
      - tab "Grid View" [selected]
      - tab "List View"
  `);
});
