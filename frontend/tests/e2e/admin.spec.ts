/**
 * admin.spec.ts — the admin / user-management prototype (#208)
 *
 * Data comes from the MSW seed store via the shared fixture, so the numbers
 * asserted here track src/mocks/admin/fixtures.ts.
 */

import { expect, test } from "./axe-test";

const NON_ADMIN_IDENTITY = {
  authenticated: true,
  username: "test.user",
  name: "Test User",
  email: "test.user@example.com",
  groups: ["/users"],
};

test.describe("entry point", () => {
  test("admins reach the users list from the profile menu", async ({ page }) => {
    await page.goto("/");
    await expect(page.locator("header").getByRole("link", { name: "Admin" })).toHaveCount(0);

    await page.getByRole("button", { name: "Account menu" }).click();
    await page.getByRole("menuitem", { name: "Administration" }).click();
    await expect(page).toHaveURL(/\/admin\/users$/);
    await expect(page.getByRole("heading", { level: 1, name: "Administration" })).toBeVisible();
    await expect(
      page.getByRole("navigation", { name: "Admin sections" }).getByRole("link", {
        name: "Users",
      }),
    ).toHaveAttribute("aria-current", "page");
  });

  test("non-admins get no link and a polite dead end", async ({ page }) => {
    await page.route(/\/api\/v1\/caller-identity/, (route) =>
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(NON_ADMIN_IDENTITY),
      }),
    );

    await page.goto("/");
    await page.getByRole("button", { name: "Account menu" }).click();
    await expect(page.getByRole("menuitem", { name: "Sign out" })).toBeVisible();
    await expect(page.getByRole("menuitem", { name: "Administration" })).toHaveCount(0);
    await page.keyboard.press("Escape");

    await page.goto("/admin/users");
    await expect(page.getByRole("status")).toContainText("don't have access");
    await expect(page.getByRole("link", { name: "Back to the Launchpad" })).toBeVisible();
  });
});

test.describe("users", () => {
  test("lists, searches and filters users", async ({ page }) => {
    await page.goto("/admin/users");
    const table = page.getByRole("table", { name: "Users" });
    await expect(table).toBeVisible();
    await expect(page.getByText("60 accounts", { exact: false })).toBeVisible();

    // 25 per page by default.
    await expect(table.getByRole("row")).toHaveCount(26);

    await page.getByRole("searchbox", { name: "Search users" }).fill("alice@");
    await expect(table.getByRole("row")).toHaveCount(2);
    await expect(table.getByRole("link", { name: /Alice Alvarez/ })).toBeVisible();

    await page.getByRole("searchbox", { name: "Search users" }).fill("nobody-here");
    await expect(page.getByRole("status").filter({ hasText: "No users match" })).toBeVisible();
    await page.getByRole("searchbox", { name: "Search users" }).fill("");

    await page.getByRole("combobox", { name: "Filter by status" }).click();
    await page.getByRole("option", { name: "Disabled" }).click();
    await expect(table.getByRole("row", { name: /Disabled/ }).first()).toBeVisible();
    await expect(table.getByRole("row")).toHaveCount(6);
  });

  test("selecting rows reveals bulk actions and applies a group change", async ({ page }) => {
    await page.goto("/admin/users");
    await page.getByRole("searchbox", { name: "Search users" }).fill("dana");
    await page.getByRole("checkbox", { name: "Select Dana Dubois" }).check();

    await page.getByRole("button", { name: "Add to group…" }).click();
    const dialog = page.getByRole("dialog", { name: "Add 1 user to a group" });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("combobox", { name: "Group" }).click();
    await page.getByRole("option", { name: "developers" }).click();
    await dialog.getByRole("button", { name: "Add to group" }).click();

    await expect(dialog).toBeHidden();
    await expect(page.getByText("Users updated")).toBeVisible();
    await expect(
      page.getByRole("table", { name: "Users" }).getByRole("link", { name: "developers" }),
    ).toBeVisible();
  });

  test("the user detail page explains access through groups", async ({ page }) => {
    await page.goto("/admin/users/usr-bruno");
    await expect(page.getByRole("heading", { level: 2, name: /Bruno Brennan/ })).toBeVisible();

    const access = page.getByRole("table", { name: "Accessible services" });
    await expect(access.getByRole("row", { name: /JupyterHub/ })).toContainText("users");
    await expect(access.getByRole("row", { name: /MLflow/ })).toContainText("data-science");
    await expect(access.getByRole("row", { name: /Platform Docs/ })).toContainText("Everyone");
    await expect(access.getByRole("row", { name: /Grafana/ })).toHaveCount(0);

    await page.getByRole("tab", { name: /^Roles/ }).click();
    await expect(page.getByText("via data-science")).toBeVisible();
  });
});

test.describe("groups", () => {
  test("creates a group and blocks deleting a referenced one", async ({ page }) => {
    await page.goto("/admin/groups");
    await page.getByRole("button", { name: "New group" }).first().click();
    const dialog = page.getByRole("dialog", { name: "New group" });
    await dialog.getByRole("textbox", { name: "Name" }).fill("data-engineering");
    await dialog.getByRole("button", { name: "Create group" }).click();
    await expect(dialog).toBeHidden();
    await expect(
      page.getByRole("table", { name: "Groups" }).getByRole("link", { name: /data-engineering/ }),
    ).toBeVisible();

    await page.goto("/admin/groups/grp-analysts");
    await page.getByRole("button", { name: "Delete group" }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Delete group" }).click();
    await expect(page.getByRole("dialog")).toContainText("still referenced by 1 service gate");
  });
});

test.describe("services", () => {
  test("shows the gate and who gets through it", async ({ page }) => {
    await page.goto("/admin/services");
    const table = page.getByRole("table", { name: "Services" });
    await expect(table.getByRole("row", { name: /Platform Docs/ })).toContainText("Everyone");
    await expect(table.getByRole("row", { name: /Status Page/ })).toContainText(
      "Any signed-in user",
    );

    await table.getByRole("link", { name: /Superset/ }).click();
    await expect(page.getByRole("heading", { level: 2, name: /Superset/ })).toBeVisible();
    await expect(page.getByText("groups:")).toBeVisible();
    await expect(page.getByRole("link", { name: "Pack settings" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Add to analysts" })).toBeVisible();
  });
});

test.describe("accessibility", () => {
  for (const path of ["/admin/users", "/admin/users/usr-alice", "/admin/services/svc-superset"]) {
    test(`${path} has no automatically detectable WCAG A/AA violations`, async ({
      page,
      makeAxeBuilder,
    }, testInfo) => {
      await page.goto(path);
      await expect(page.getByRole("heading", { level: 1, name: "Administration" })).toBeVisible();
      await expect(page.getByRole("heading", { level: 2 })).toBeVisible();

      const results = await makeAxeBuilder().analyze();
      await testInfo.attach(`axe-${path.replaceAll("/", "_")}`, {
        body: JSON.stringify(results, null, 2),
        contentType: "application/json",
      });
      expect(results.violations).toEqual([]);
    });
  }
});
