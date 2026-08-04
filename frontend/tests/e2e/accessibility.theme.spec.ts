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
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(38, 38, 40)");
  await expect(page.locator("body")).toHaveCSS("color", "rgb(248, 248, 248)");
  await expect(page.locator("header")).toHaveCSS("background-color", "rgb(53, 53, 56)");
  await expect(page.locator("header")).toHaveCSS("border-bottom-color", "rgb(90, 90, 97)");
  await expect(page.getByText("Pinned services", { exact: true })).toHaveCSS(
    "color",
    "rgb(183, 183, 187)",
  );
  await expect(page.getByText("Quick access to your most-used tools", { exact: true })).toHaveCSS(
    "color",
    "rgb(157, 157, 166)",
  );

  // The theme toggle now lives inside the profile menu as a radio group.
  await page.getByRole("button", { name: /account menu/i }).click();
  const darkOption = page.getByRole("menuitemradio", { name: /dark mode/i });
  const themeGroup = page.getByRole("group", { name: "Theme" });
  await expect(darkOption).toBeVisible();
  await expect(darkOption).toHaveAttribute("aria-checked", "true");
  await expect(themeGroup).toHaveCSS("height", "34px");
  await expect(themeGroup).toHaveCSS("gap", "4px");
  await expect(themeGroup).toHaveCSS("padding", "4px");
  await expect(themeGroup).toHaveCSS("border-radius", "8px");
  await expect(darkOption).toHaveCSS("padding", "2px 6px");
  await expect(darkOption).toHaveCSS("border-radius", "6px");

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
