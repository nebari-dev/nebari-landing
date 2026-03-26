import { test, expect } from "./fixtures";

test("header accessibility tree stays stable", async ({ page }) => {
  await page.goto("/");

  const header = page.locator("header");
  await expect(header).toBeVisible();

  await expect(header).toMatchAriaSnapshot(`
    - link "Go to homepage"
    - button /Switch to (dark|light) mode/
    - button "Notifications"
  `);
});

test("services controls accessibility tree stays stable", async ({ page }) => {
  await page.goto("/");

  const allServicesRegion = page.getByRole("region", { name: /All services/i });
  await expect(allServicesRegion).toBeVisible();

  await expect(allServicesRegion).toMatchAriaSnapshot(`
    - textbox "Search"
    - button "Search"
    - group:
      - radio "Table view" [checked]
      - radio "Grid view"
  `);
});
