import type { Locator } from "@playwright/test";
import { expect, test } from "./fixtures/e2e";
import { simpleServices } from "./fixtures/services";

const cardNames = (region: Locator) =>
  region
    .getByRole("link", { name: /opens in a new tab/ })
    .evaluateAll((links) =>
      links.map((link) => link.getAttribute("aria-label")?.replace(" (opens in a new tab)", "")),
    );

test.describe("simple layout (fewer than 6 services)", () => {
  test.use({ services: [simpleServices, { scope: "test" }] });

  test("renders one grid without search, filter, sort, view, or pin controls", async ({
    page,
  }) => {
    await page.goto("/");

    const main = page.getByRole("main");
    await expect(page.getByRole("heading", { level: 1 })).toHaveText("Welcome back, Test");

    await expect(page.getByRole("textbox", { name: "Search services" })).toHaveCount(0);
    await expect(page.getByRole("combobox")).toHaveCount(0);
    await expect(page.getByRole("group", { name: "View" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^(Pinned|All services)/ })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /pin service/i })).toHaveCount(0);

    await expect
      .poll(() => cardNames(main))
      .toEqual(simpleServices.map((service) => service.name));
  });

  test("the whole card opens the service in a new tab", async ({ page }) => {
    await page.goto("/");

    const card = page.getByRole("link", { name: "JupyterHub (opens in a new tab)" });
    await expect(card).toHaveAttribute("href", "https://example.com/jupyterhub");
    await expect(card).toHaveAttribute("target", "_blank");
  });
});

test.describe("richer layout (6 or more services)", () => {
  test("category filter narrows both sections", async ({ page }) => {
    await page.goto("/");

    await page.getByRole("combobox", { name: "Filter by category" }).click();
    await page.getByRole("option", { name: "Monitoring" }).click();
    await page.keyboard.press("Escape");

    await expect(page.getByRole("button", { name: "Pinned 0" })).toBeVisible();
    await expect(page.getByRole("button", { name: "All services 1" })).toBeVisible();
    const allServices = page.getByRole("region", { name: /^All services/ });
    await expect.poll(() => cardNames(allServices)).toEqual(["Grafana"]);
  });

  test("search narrows the services", async ({ page }) => {
    await page.goto("/");

    await page.getByRole("textbox", { name: "Search services" }).fill("git");

    const allServices = page.getByRole("region", { name: /^All services/ });
    await expect.poll(() => cardNames(allServices)).toEqual(["Argo CD"]);
  });

  test("sort dropdown reorders the cards", async ({ page }) => {
    await page.goto("/");

    const allServices = page.getByRole("region", { name: /^All services/ });
    await expect.poll(() => cardNames(allServices)).toEqual([
      "Argo CD",
      "Dask Gateway",
      "Grafana",
      "MLflow",
      "Nebi",
    ]);

    await page.getByRole("combobox", { name: "Sort services" }).click();
    await page.getByRole("option", { name: "Name (Z–A)" }).click();

    await expect(page.getByRole("combobox", { name: "Sort services" })).toContainText(
      "Sort: Name (Z–A)",
    );
    await expect.poll(() => cardNames(allServices)).toEqual([
      "Nebi",
      "MLflow",
      "Grafana",
      "Dask Gateway",
      "Argo CD",
    ]);
  });

  test("table lists pinned rows first and its headers drive the sort dropdown", async ({
    page,
  }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Table" }).click();

    const table = page.getByRole("table", { name: "Services" });
    const rowNames = () => cardNames(table);
    const sortSelect = page.getByRole("combobox", { name: "Sort services" });
    const categoryHeader = table.getByRole("columnheader", { name: "Category" });

    await expect.poll(() => rowNames()).toEqual([
      "JupyterHub",
      "Argo CD",
      "Dask Gateway",
      "Grafana",
      "MLflow",
      "Nebi",
    ]);

    await categoryHeader.getByRole("button").click();
    await expect(sortSelect).toContainText("Sort: Category");
    await expect(categoryHeader).toHaveAttribute("aria-sort", "ascending");

    await categoryHeader.getByRole("button").click();
    await expect(sortSelect).toContainText("Sort: Category (Z–A)");
    await expect(categoryHeader).toHaveAttribute("aria-sort", "descending");
    await expect.poll(() => rowNames()).toEqual([
      "JupyterHub",
      "Argo CD",
      "Grafana",
      "MLflow",
      "Nebi",
      "Dask Gateway",
    ]);
  });
});
