import { test, expect } from "./axe-test";

test("light theme has no automatically detectable WCAG A/AA violations", async ({
  page,
  makeAxeBuilder,
}, testInfo) => {
  await page.goto("/");
  await expect(page.getByRole("banner")).toBeVisible();

  const results = await makeAxeBuilder().analyze();

  await testInfo.attach("axe-light-results", {
    body: JSON.stringify(results, null, 2),
    contentType: "application/json",
  });

  expect(results.violations).toEqual([]);
});
