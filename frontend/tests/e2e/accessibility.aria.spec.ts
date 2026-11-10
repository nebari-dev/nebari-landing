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

  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Welcome back, Test");
  await expect(page.getByRole("textbox", { name: "Search services" })).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Filter by category" })).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Sort services" })).toContainText("Sort: Default");
  await expect(page.getByRole("group", { name: "View" })).toMatchAriaSnapshot(`
    - group "View":
      - button "Grid" [pressed]
      - button "Table"
  `);
  await expect(page.getByRole("button", { name: "Pinned 1" })).toBeVisible();
  await expect(page.getByRole("button", { name: "All services 5" })).toBeVisible();
});
