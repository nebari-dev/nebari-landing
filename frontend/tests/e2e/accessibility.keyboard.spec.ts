import { test, expect } from "./fixtures";

test("header controls are reachable by keyboard", async ({ page }) => {
  await page.goto("/");

  await expect(
    page.getByRole("button", { name: /notifications/i })
  ).toBeVisible();

  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: /go to homepage/i })
  ).toBeFocused();

  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("button", { name: /switch to dark mode|switch to light mode/i })
  ).toBeFocused();

  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("button", { name: /notifications/i })
  ).toBeFocused();
});

test("Ctrl+K focuses the search input", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByPlaceholder("Search")).toBeVisible();

  await page.keyboard.press("ControlOrMeta+K");
  await expect(page.getByPlaceholder("Search")).toBeFocused();
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
