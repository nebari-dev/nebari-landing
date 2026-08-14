import { expect, test } from "./fixtures/e2e";

test("dark table rows expose a visible hover surface", async ({ page }) => {
  await page.goto("/");
  await page.evaluate(() => document.documentElement.classList.add("dark"));

  const allServices = page.getByRole("region", { name: /All services/i });
  await allServices.getByRole("tab", { name: /List View/i }).click();
  const serviceRow = allServices
    .locator('[data-slot="table-container"]')
    .getByRole("link", { name: /JupyterHub/i });
  const beforeHover = await serviceRow.evaluate(
    (element) => getComputedStyle(element).backgroundColor,
  );

  await serviceRow.hover();
  await expect(serviceRow).not.toHaveCSS("background-color", beforeHover);
});

test("table header remains visually distinct from the table body in both themes", async ({
  page,
}) => {
  const openTable = async () => {
    await page.goto("/");
    const allServicesRegion = page.getByRole("region", { name: /All services/i });
    await allServicesRegion.getByRole("tab", { name: /List View/i }).click();
  };

  await page.goto("/");
  await page.evaluate(() => {
    window.localStorage.setItem("launchpad:themeMode", "light");
  });
  await openTable();

  const table = page.locator('[data-slot="table"]');
  const headerCell = page.locator('[data-slot="table-head"]').first();
  const readBackground = (element: HTMLElement | SVGElement) =>
    getComputedStyle(element).backgroundColor;

  const lightHeader = await headerCell.evaluate(readBackground);
  const lightBody = await table.evaluate(readBackground);
  expect(lightHeader).not.toBe(lightBody);

  await page.evaluate(() => {
    window.localStorage.setItem("launchpad:themeMode", "dark");
  });
  await openTable();
  await expect(page.locator("html")).toHaveClass(/dark/);

  const darkHeader = await headerCell.evaluate(readBackground);
  const darkBody = await table.evaluate(readBackground);
  expect(darkHeader).not.toBe(darkBody);
});

test("accordion sections do not add a divider between content groups", async ({ page }) => {
  await page.goto("/");

  const items = page.locator('[data-slot="accordion-item"]');
  await expect(items).toHaveCount(2);

  const dividerWidths = await items.evaluateAll((elements) =>
    elements.map((element) => Number.parseFloat(getComputedStyle(element).borderBottomWidth)),
  );
  expect(dividerWidths[0]).toBe(dividerWidths[1]);
});

test("pinned service card title is not underlined", async ({ page }) => {
  await page.goto("/");

  const pinnedServices = page.getByRole("region", { name: /Pinned services/i });
  const pinnedService = pinnedServices.getByRole("link", { name: /JupyterHub/i });

  await expect(pinnedService).toHaveCSS("text-decoration-line", "none");
  await pinnedService.hover();
  await expect(pinnedService).toHaveCSS("text-decoration-line", "none");
});
