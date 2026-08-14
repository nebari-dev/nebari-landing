import { expect, test } from "./fixtures/e2e";

test("account dropdown opens and closes with keyboard", async ({ page }) => {
  await page.goto("/");

  const trigger = page.getByRole("button", { name: /account menu/i });
  await trigger.focus();
  await page.keyboard.press("Enter");

  await expect(page.getByRole("menu")).toBeVisible();
  await expect(page.getByRole("group", { name: "Theme" })).toBeVisible();

  await page.keyboard.press("Escape");
  await expect(page.getByRole("menu")).not.toBeVisible();
  await expect(trigger).toBeFocused();
});

test("sign out action uses body-small typography", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: /account menu/i }).click();

  const signOut = page.getByRole("menuitem", { name: /sign out/i });
  await expect(signOut).toHaveCSS("color", "rgb(210, 22, 28)");
  await expect(signOut).toHaveCSS("font-family", /Geist Variable/);
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
  await expect(accountMenu).toHaveCSS("padding-top", "4px");
  await expect(accountMenu).toHaveCSS("padding-bottom", "4px");
});
