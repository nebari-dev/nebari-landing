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

test("services toolbar stays on one row and keeps search text sizing stable", async ({ page }) => {
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
    const searchButton = allServicesRegion.getByRole("button", {
      name: /^Search$/,
    });
    const viewToggle = allServicesRegion.getByRole("radiogroup");

    const inputBox = await getBox(searchInput);
    const buttonBox = await getBox(searchButton);
    const toggleBox = await getBox(viewToggle);

    const inputCenterY = inputBox.y + inputBox.height / 2;
    const toggleCenterY = toggleBox.y + toggleBox.height / 2;

    expect(Math.abs(inputCenterY - toggleCenterY)).toBeLessThanOrEqual(2);
    expect(buttonBox.x + buttonBox.width).toBeLessThanOrEqual(toggleBox.x);

    await expect(searchInput).toHaveCSS("font-size", "14px");
    await expect(searchInput).toHaveCSS("line-height", "20px");
    await expect(searchInput).toHaveCSS("height", "46px");
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
    await allServicesRegion.getByRole("radio", { name: /Table view/i }).click();

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
