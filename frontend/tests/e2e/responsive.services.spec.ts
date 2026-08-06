import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures/e2e";

const responsiveServices = [
  {
    id: "svc-long",
    name: "Long Running Analytics Workspace",
    status: "Healthy",
    description:
      "Notebook workspace for collaborative data analysis with shared compute environments and a description that is intentionally long enough to wrap across two lines.",
    category: ["Data Science"],
    pinned: true,
    image: "",
    url: "https://example.com/analytics",
  },
  {
    id: "svc-monitoring",
    name: "Grafana",
    status: "Unknown",
    description: "Metrics dashboards for platform health.",
    category: ["Observability"],
    pinned: false,
    image: "",
    url: "https://example.com/grafana",
  },
];

async function mockResponsiveServices(page: Page) {
  await page.route(/\/api\/.*services(?:\/)?(?:\?.*)?$/, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(responsiveServices),
    });
  });
}

async function getBox(locator: Locator) {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error("Expected locator to have a bounding box");
  }
  return box;
}

test("service search matches the responsive card width", async ({ page }) => {
  await mockResponsiveServices(page);

  const widths = [560, 640, 767, 816, 1024];

  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");

    const allServicesRegion = page.getByRole("region", {
      name: /All services/i,
    });
    await expect(allServicesRegion).toBeVisible();

    const searchInput = allServicesRegion.getByPlaceholder("Search");
    const viewToggle = allServicesRegion.getByRole("tablist");
    const firstCard = allServicesRegion.getByRole("link", {
      name: /Long Running Analytics Workspace.*opens in a new tab/i,
    });

    const inputBox = await getBox(searchInput);
    const toggleBox = await getBox(viewToggle);
    const cardBox = await getBox(firstCard);

    expect(Math.abs(inputBox.width - cardBox.width)).toBeLessThanOrEqual(1);

    if (width >= 640) {
      const inputCenterY = inputBox.y + inputBox.height / 2;
      const toggleCenterY = toggleBox.y + toggleBox.height / 2;
      expect(Math.abs(inputCenterY - toggleCenterY)).toBeLessThanOrEqual(2);
    } else {
      expect(toggleBox.y).toBeGreaterThanOrEqual(inputBox.y + inputBox.height);
    }

    await expect(searchInput).toHaveCSS("font-size", "14px");
    await expect(searchInput).toHaveCSS("line-height", "20px");
    await expect(searchInput).toHaveCSS("height", "34px");
    await expect(viewToggle).toHaveCSS("height", "34px");
  }
});

test("services table keeps headers and rows in bounds while resizing", async ({ page }) => {
  await mockResponsiveServices(page);

  const widths = [700, 767, 816, 1024];

  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");

    const allServicesRegion = page.getByRole("region", {
      name: /All services/i,
    });
    await expect(allServicesRegion).toBeVisible();

    // Grid is the default view, so switch to the table view this test exercises.
    await allServicesRegion.getByRole("tab", { name: /List View/i }).click();

    const tableContainer = page.locator('[data-slot="table-container"]').first();
    const actionsHeader = page.getByRole("columnheader", { name: /Actions/i }).getByText("Actions");
    const pinButton = allServicesRegion.getByRole("button", {
      name: /^Unpin service$/,
    });
    const longDescription = page.getByText(/Notebook workspace for collaborative data analysis/);

    await expect(actionsHeader).toBeVisible();
    await expect(pinButton).toBeVisible();
    await expect(longDescription).toBeVisible();

    const containerBox = await getBox(tableContainer);
    const actionsHeaderBox = await getBox(actionsHeader);
    const pinButtonBox = await getBox(pinButton);

    const containerRight = containerBox.x + containerBox.width;

    expect(actionsHeaderBox.x).toBeGreaterThanOrEqual(containerBox.x - 1);
    expect(actionsHeaderBox.x + actionsHeaderBox.width).toBeLessThanOrEqual(containerRight + 1);
    expect(pinButtonBox.x + pinButtonBox.width).toBeLessThanOrEqual(containerRight + 1);

    const overflow = await tableContainer.evaluate((element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }));

    expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1);

    const descriptionMetrics = await longDescription.evaluate((element) => {
      const styles = window.getComputedStyle(element);
      return {
        height: element.getBoundingClientRect().height,
        lineHeight: Number.parseFloat(styles.lineHeight),
      };
    });

    expect(descriptionMetrics.height).toBeLessThanOrEqual(descriptionMetrics.lineHeight * 2 + 1);
  }
});

test("services table uses the semantic light and dark surface colors", async ({ page }) => {
  await mockResponsiveServices(page);

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

  const searchInput = page.getByRole("textbox", { name: "Search services" });
  const viewToggle = page.getByRole("tablist");
  const activeView = page.getByRole("tab", { name: /List View/i });
  const container = page.locator('[data-slot="table-container"]');
  const table = page.locator('[data-slot="table"]');
  const header = page.locator('[data-slot="table-header"]');
  const headerCell = page.locator('[data-slot="table-head"]').first();

  await expect(header).toHaveCSS("background-color", "oklch(0.9494 0.0013 286.37)");
  await expect(headerCell).toHaveCSS("background-color", "oklch(0.9494 0.0013 286.37)");
  await expect(container).toHaveCSS("border-color", "oklch(0.7806 0.0056 286.27)");
  await expect(searchInput).toHaveCSS("background-color", "oklch(1 0 0)");
  await expect(viewToggle).toHaveCSS("background-color", "oklch(0.9494 0.0013 286.37)");
  await expect(viewToggle).toHaveCSS("border-radius", "8px");
  await expect(viewToggle).toHaveCSS("gap", "4px");
  await expect(viewToggle).toHaveCSS("padding", "4px");
  await expect(activeView).toHaveCSS("background-color", "oklch(1 0 0)");
  await expect(activeView).toHaveCSS("border-radius", "6px");
  await expect(activeView).toHaveCSS("border-width", "1px");
  await expect(activeView).toHaveCSS("padding", "2px 6px");

  await page.evaluate(() => {
    window.localStorage.setItem("launchpad:themeMode", "dark");
  });
  await openTable();

  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(header).toHaveCSS("background-color", "rgb(38, 38, 40)");
  await expect(headerCell).toHaveCSS("background-color", "rgb(38, 38, 40)");
  await expect(container).toHaveCSS("border-color", "oklch(0.4701 0.0112 285.96)");
  await expect(searchInput).toHaveCSS("background-color", "rgb(53, 53, 56)");
  await expect(viewToggle).toHaveCSS("background-color", "oklch(0.3301 0.0052 286.11)");
  await expect(activeView).toHaveCSS("background-color", "rgb(53, 53, 56)");

  const categoryBadge = page.getByText("Observability", { exact: true });
  const unknownBadge = page.getByText("Unknown", { exact: true });
  await expect(categoryBadge).toHaveCSS("background-color", "rgb(71, 71, 75)");
  await expect(categoryBadge).toHaveCSS("color", "rgb(183, 183, 187)");
  await expect(unknownBadge).toHaveCSS("background-color", "rgb(71, 71, 75)");
  await expect(unknownBadge).toHaveCSS("color", "rgb(157, 157, 166)");

  const surfaces = await Promise.all([
    container.evaluate((element) => getComputedStyle(element).backgroundColor),
    table.evaluate((element) => getComputedStyle(element).backgroundColor),
  ]);
  expect(surfaces[1]).toBe(surfaces[0]);
});
