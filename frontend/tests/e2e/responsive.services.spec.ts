import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures/e2e";
import { richerServices } from "./fixtures/services";

const longService = {
  id: "svc-long",
  name: "Long Running Analytics Workspace",
  status: "Healthy",
  description:
    "Notebook workspace for collaborative data analysis with shared compute environments and a description that is long enough to wrap.",
  category: ["Data Science"],
  pinned: true,
  image: "",
  url: "https://example.com/analytics",
};

test.use({
  services: [
    [longService, ...richerServices.filter((service) => !service.pinned)],
    { scope: "test" },
  ],
});

const widths = [390, 560, 768, 1024, 1440];

const hasHorizontalScroll = (page: Page) =>
  page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);

async function getBox(locator: Locator) {
  const box = await locator.boundingBox();
  if (!box) {
    throw new Error("Expected locator to have a bounding box");
  }
  return box;
}

test("service controls stay on screen at every width", async ({ page }) => {
  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");

    for (const control of [
      page.getByRole("textbox", { name: "Search services" }),
      page.getByRole("combobox", { name: "Filter by category" }),
      page.getByRole("combobox", { name: "Sort services" }),
      page.getByRole("button", { name: "Grid" }),
      page.getByRole("button", { name: "Table" }),
      page.getByRole("link", { name: "Long Running Analytics Workspace (opens in a new tab)" }),
    ]) {
      await expect(control, `at ${width}px`).toBeInViewport();
    }
    expect(await hasHorizontalScroll(page), `at ${width}px`).toBe(false);
  }
});

test("table row controls stay reachable at every width", async ({ page }) => {
  for (const width of widths) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    await page.getByRole("button", { name: "Table" }).click();

    const table = page.getByRole("table", { name: "Services" });
    const row = table.getByRole("row", { name: /Long Running Analytics Workspace/ });

    for (const control of [
      row.getByRole("button", { name: "Unpin service" }),
      row.getByRole("link", { name: /Long Running Analytics Workspace/ }),
      row.getByRole("button", { name: "Long Running Analytics Workspace details" }),
    ]) {
      await control.scrollIntoViewIfNeeded();
      await expect(control, `at ${width}px`).toBeInViewport();
    }
    expect(await hasHorizontalScroll(page), `at ${width}px`).toBe(false);
  }
});

test("service controls preserve their responsive order and stay in bounds", async ({ page }) => {
  for (const width of [560, 768, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");

    const content = page.getByRole("heading", { level: 1 }).locator("..");
    const searchInput = page.getByRole("textbox", { name: "Search services" });
    const categoryFilter = page.getByRole("combobox", { name: "Filter by category" });
    const sortSelect = page.getByRole("combobox", { name: "Sort services" });
    const viewToggle = page.getByRole("group", { name: "View" });
    const firstCard = page.getByRole("link", {
      name: /Long Running Analytics Workspace.*opens in a new tab/i,
    });

    const inputBox = await getBox(searchInput);
    const filterBox = await getBox(categoryFilter);
    const sortBox = await getBox(sortSelect);
    const toggleBox = await getBox(viewToggle);
    const cardBox = await getBox(firstCard);
    const contentBox = await getBox(content);
    const contentRight = contentBox.x + contentBox.width;

    for (const box of [inputBox, filterBox, sortBox, toggleBox, cardBox]) {
      expect(box.x, `at ${width}px`).toBeGreaterThanOrEqual(Math.floor(contentBox.x));
      expect(box.x + box.width, `at ${width}px`).toBeLessThanOrEqual(Math.ceil(contentRight));
    }

    expect(cardBox.y, `at ${width}px`).toBeGreaterThanOrEqual(
      Math.max(...[inputBox, filterBox, sortBox, toggleBox].map((box) => box.y + box.height)),
    );

    if (width >= 640) {
      const controlsOverlapStart = Math.max(inputBox.y, toggleBox.y);
      const controlsOverlapEnd = Math.min(
        inputBox.y + inputBox.height,
        toggleBox.y + toggleBox.height,
      );
      expect(controlsOverlapStart, `at ${width}px`).toBeLessThan(controlsOverlapEnd);
    } else {
      expect(toggleBox.y, `at ${width}px`).toBeGreaterThanOrEqual(inputBox.y + inputBox.height);
    }
  }
});

test("services table keeps headers and rows in bounds while resizing", async ({ page }) => {
  for (const width of [700, 767, 816, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");

    // Grid is the default view, so switch to the table view this test exercises.
    await page.getByRole("button", { name: "Table" }).click();

    const tableContainer = page.locator('[data-slot="table-container"]').first();
    const statusHeader = page.getByRole("columnheader", { name: "Status" });
    const longServiceRow = page
      .getByRole("table", { name: "Services" })
      .getByRole("row", { name: /Long Running Analytics Workspace/ });
    const pinButton = longServiceRow.getByRole("button", { name: /^Unpin service$/ });
    // Service cell: an icon beside the name and description.
    const serviceIcon = longServiceRow
      .getByRole("cell")
      .nth(1)
      .locator(":scope > div > div")
      .first();
    const serviceTitle = longServiceRow.getByRole("link", {
      name: /Long Running Analytics Workspace/,
    });
    const longDescription = longServiceRow.getByText(
      /Notebook workspace for collaborative data analysis/,
    );

    await expect(statusHeader).toBeVisible();
    await expect(pinButton).toBeVisible();
    await expect(longDescription).toBeVisible();
    // The scroll container is a tab stop only while its content overflows.
    const isScrollable = await tableContainer.evaluate(
      (element) => element.scrollWidth > element.clientWidth,
    );
    if (isScrollable) {
      await expect(tableContainer).toHaveAttribute("tabindex", "0");
      await tableContainer.focus();
      await expect(tableContainer).toBeFocused();
    } else {
      await expect(tableContainer).not.toHaveAttribute("tabindex");
    }

    const iconBox = await getBox(serviceIcon);
    const titleBox = await getBox(serviceTitle);
    const descriptionBox = await getBox(longDescription);

    const textBlockTop = titleBox.y;
    const textBlockBottom = descriptionBox.y + descriptionBox.height;
    const iconCenter = iconBox.y + iconBox.height / 2;
    expect(descriptionBox.y, `at ${width}px`).toBeGreaterThanOrEqual(titleBox.y + titleBox.height);
    expect(iconCenter, `at ${width}px`).toBeGreaterThanOrEqual(textBlockTop);
    expect(iconCenter, `at ${width}px`).toBeLessThanOrEqual(textBlockBottom);

    expect(
      await tableContainer.evaluate((element) => element.scrollWidth > element.clientWidth),
      `at ${width}px`,
    ).toBe(false);
  }
});
