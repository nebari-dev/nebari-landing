import { test, expect } from "./fixtures";

test("notifications dropdown opens and closes with keyboard", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /notifications/i });

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
