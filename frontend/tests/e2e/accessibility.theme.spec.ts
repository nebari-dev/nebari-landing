import { expect, test } from "./axe-test";

test("dark theme loads correctly and has no detectable accessibility violations", async ({
  page,
  makeAxeBuilder,
}, testInfo) => {
  await page.addInitScript(() => {
    window.localStorage.setItem("launchpad:themeMode", "dark");
  });

  await page.goto("/");

  await expect(page.locator("html")).toHaveClass(/dark/);

  // The theme toggle now lives inside the profile menu as a radio group.
  await page.getByRole("button", { name: /account menu/i }).click();
  const darkOption = page.getByRole("menuitemradio", { name: /dark mode/i });
  const themeGroup = page.getByRole("group", { name: "Theme" });
  await expect(darkOption).toBeVisible();
  await expect(darkOption).toHaveAttribute("aria-checked", "true");
  await expect(themeGroup).toContainText("Light");
  await expect(themeGroup).toContainText("Dark");
  await expect(themeGroup).toContainText("System");

  // Let any entrance animation settle so axe evaluates the final rendered state.
  const menuContent = page.locator('[data-slot="dropdown-menu-content"]');
  await expect(menuContent).toBeVisible();
  await menuContent.evaluate(async (element) => {
    await Promise.all(
      element
        .getAnimations({ subtree: true })
        .map((animation) => animation.finished.catch(() => undefined)),
    );
  });

  // Base UI injects focus-guard sentinels around portaled popups. They are
  // implementation-only nodes; keep the open menu in scope while excluding
  // those sentinels from axe's aria-hidden-focus rule.
  const results = await makeAxeBuilder().exclude("[data-base-ui-focus-guard]").analyze();

  await testInfo.attach("axe-dark-results", {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  });

  expect(results.violations).toEqual([]);
});
