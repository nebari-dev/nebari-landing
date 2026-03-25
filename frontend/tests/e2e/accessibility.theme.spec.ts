import { test, expect } from "./axe-test";

test("dark theme loads correctly and has no detectable accessibility violations", async ({
  page,
  makeAxeBuilder,
}, testInfo) => {
  await page.addInitScript(() => {
    window.localStorage.setItem("launchpad:isDarkMode", "true");
  });

  await page.goto("/");

  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(
    page.getByRole("button", { name: /switch to light mode/i })
  ).toBeVisible();

  const results = await makeAxeBuilder().analyze();

  await testInfo.attach("axe-dark-results", {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  }); 

  expect(results.violations).toEqual([]);
});
