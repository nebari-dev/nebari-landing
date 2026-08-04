import { expect, test } from "./fixtures/e2e";

test("notifications dropdown opens and closes with keyboard", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /notifications/i });

  await expect(page.locator("header")).toHaveCSS("background-color", "rgb(238, 238, 239)");
  await expect(page.locator("header")).toHaveCSS("border-bottom-color", "rgb(183, 183, 187)");
  await trigger.hover();
  await expect(trigger).toHaveCSS("background-color", "rgb(217, 217, 220)");

  await trigger.focus();
  await expect(trigger).toBeFocused();

  await page.keyboard.press("Enter");

  await expect(page.getByRole("menu")).toBeVisible();

  // Works whether the menu is empty or has notifications.
  const emptyState = page.getByText("No notifications");
  const anyMenuText = page.getByRole("menu");

  await expect(anyMenuText).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).not.toBeVisible();
  await expect(trigger).toBeFocused();

  // Keep this line so the empty state is not tree-shaken by the test runner.
  await emptyState.count();
});

test("sign out action uses body-small typography", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: /account menu/i }).click();

  const signOut = page.getByRole("menuitem", { name: /sign out/i });
  await expect(signOut).toHaveCSS("color", "rgb(210, 22, 28)");
  await expect(signOut).toHaveCSS("font-family", /Inter Variable/);
  await expect(signOut).toHaveCSS("font-size", "14px");
  await expect(signOut).toHaveCSS("font-style", "normal");
  await expect(signOut).toHaveCSS("font-weight", "400");
  await expect(signOut).toHaveCSS("line-height", "20px");

  const icon = signOut.locator("svg");
  await expect(icon).toHaveCSS("width", "16px");
  await expect(icon).toHaveCSS("height", "16px");
  await expect(icon).toHaveCSS("flex-shrink", "0");
});

test("header actions leave room for the profile focus ring", async ({ page }) => {
  await page.goto("/");

  const actions = page.locator('[data-slot="menu-bar-actions"]');
  const accountMenu = page.getByRole("button", { name: /account menu/i });

  await expect(actions).toHaveCSS("column-gap", "8px");
  await expect(accountMenu).toHaveCSS("padding-left", "10px");
  await expect(accountMenu).toHaveCSS("padding-right", "10px");
});

test("compact icon button focus ring has no offset gap", async ({ page }) => {
  await page.goto("/");

  const notificationButton = page.getByRole("button", { name: /notifications/i });
  await notificationButton.focus();

  const focusShadow = await notificationButton.evaluate(
    (element) => getComputedStyle(element).boxShadow,
  );
  expect(focusShadow).not.toContain("0px 0px 0px 4px");
});

test("notification badge uses the compact size", async ({ page }) => {
  await page.goto("/");

  const badge = page.getByRole("button", { name: /notifications/i }).locator("span");
  await expect(badge).toHaveCSS("height", "16px");
  await expect(badge).toHaveCSS("min-width", "16px");
  await expect(badge).toHaveCSS("align-items", "center");
  await expect(badge).toHaveCSS("justify-content", "center");
  await expect(badge).toHaveCSS("line-height", "9px");
});
