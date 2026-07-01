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
  await expect(darkOption).toBeVisible();
  await expect(darkOption).toHaveAttribute("aria-checked", "true");

  // The menu fades in (fade-in-0, duration-100). Wait for it to reach full
  // opacity so axe measures the settled colors rather than the mid-animation
  // composite, which reports false color-contrast violations.
  const menuContent = page.locator('[data-slot="dropdown-menu-content"]');
  await expect(menuContent).toHaveCSS("opacity", "1");

  const results = await makeAxeBuilder().analyze();

  await testInfo.attach("axe-dark-results", {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  });

  expect(results.violations).toEqual([]);
});
