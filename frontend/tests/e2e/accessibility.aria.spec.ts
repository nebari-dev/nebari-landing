import { expect, test } from "./fixtures/e2e";

test("header accessibility tree stays stable", async ({ page }) => {
  await page.goto("/");

  const header = page.locator("header");
  await expect(header).toBeVisible();

  // The theme toggle moved into the account menu, so the header now exposes
  // the notifications and account-menu triggers next to the logo.
  await expect(header).toMatchAriaSnapshot(`
    - link "Go to homepage"
    - button "Notifications"
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
