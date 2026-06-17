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

  // The theme toggle now lives inside the profile menu.
  await page.getByRole("button", { name: /account menu/i }).click();
  const darkOption = page.getByRole("button", { name: /dark mode/i });
  await expect(darkOption).toBeVisible();
  await expect(darkOption).toHaveAttribute("aria-pressed", "true");

  const results = await makeAxeBuilder().analyze();

  await testInfo.attach("axe-dark-results", {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  });

  expect(results.violations).toEqual([]);
});
