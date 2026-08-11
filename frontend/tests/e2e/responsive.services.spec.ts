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
  await page.route(
    /^https?:\/\/[^/]+\/api\/v1\/services\/?(?:\?.*)?$/,
    async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify(responsiveServices),
      });
    },
  );
}

async function getBox(locator: Locator) {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error("Expected locator to have a bounding box");
  }
  return box;
}

test("service controls preserve their responsive order and stay in bounds", async ({ page }) => {
  await mockResponsiveServices(page);

  const widths = [560, 768, 1024];

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
    const regionBox = await getBox(allServicesRegion);
    const regionRight = regionBox.x + regionBox.width;

    for (const box of [inputBox, toggleBox, cardBox]) {
      expect(box.x).toBeGreaterThanOrEqual(Math.floor(regionBox.x));
      expect(box.x + box.width).toBeLessThanOrEqual(Math.ceil(regionRight));
    }

    expect(cardBox.y).toBeGreaterThanOrEqual(
      Math.max(inputBox.y + inputBox.height, toggleBox.y + toggleBox.height),
    );

    if (width >= 640) {
      const controlsOverlapStart = Math.max(inputBox.y, toggleBox.y);
      const controlsOverlapEnd = Math.min(
        inputBox.y + inputBox.height,
        toggleBox.y + toggleBox.height,
      );
      expect(controlsOverlapStart).toBeLessThan(controlsOverlapEnd);
    } else {
      expect(toggleBox.y).toBeGreaterThanOrEqual(inputBox.y + inputBox.height);
    }
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
    const longServiceRow = allServicesRegion.getByRole("link", {
      name: /Long Running Analytics Workspace.*opens in a new tab/i,
    });
    const serviceIcon = longServiceRow.locator("img").first().locator("..");
    const serviceTitle = longServiceRow.getByText("Long Running Analytics Workspace");
    const longDescription = longServiceRow.getByText(
      /Notebook workspace for collaborative data analysis/,
    );

    await expect(actionsHeader).toBeVisible();
    await expect(pinButton).toBeVisible();
    await expect(longDescription).toBeVisible();
    await tableContainer.focus();
    await expect(tableContainer).toBeFocused();

    const iconBox = await getBox(serviceIcon);
    const titleBox = await getBox(serviceTitle);
    const descriptionBox = await getBox(longDescription);

    const textBlockTop = titleBox.y;
    const textBlockBottom = descriptionBox.y + descriptionBox.height;
    const iconCenter = iconBox.y + iconBox.height / 2;
    expect(descriptionBox.y).toBeGreaterThanOrEqual(titleBox.y + titleBox.height);
    expect(iconCenter).toBeGreaterThanOrEqual(textBlockTop);
    expect(iconCenter).toBeLessThanOrEqual(textBlockBottom);

    expect(
      await tableContainer.evaluate((element) => element.scrollWidth > element.clientWidth),
    ).toBe(false);
  }
});
