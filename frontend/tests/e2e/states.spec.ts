import type { Page } from "@playwright/test";
import { expect, test } from "./axe-test";
import { SERVICES_URL } from "./fixtures/e2e";

/** Holds the services request until the returned `release` is called. */
async function holdServices(page: Page) {
  let release = () => {};
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(SERVICES_URL, async (route) => {
    await held;
    await route.fallback();
  });
  return release;
}

const skeletonCards = (page: Page) => page.locator('[data-slot="card"][aria-hidden="true"]');

test.describe("loading", () => {
  test("a first visit shows the simple skeleton grid until the services arrive", async ({
    page,
  }) => {
    const release = await holdServices(page);
    await page.goto("/");

    await expect(page.getByRole("status")).toHaveText("Loading services…");
    await expect(skeletonCards(page)).toHaveCount(5);
    await expect(page.getByRole("textbox", { name: "Search services" })).toHaveCount(0);

    release();

    await expect(skeletonCards(page)).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Pinned 1" })).toBeVisible();
  });

  test("a reload shows the skeleton for the layout the services last used", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("button", { name: "All services 5" })).toBeVisible();

    const release = await holdServices(page);
    await page.reload();

    await expect(page.getByRole("textbox", { name: "Search services" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Pinned", exact: true })).toBeVisible();
    await expect(page.getByRole("button", { name: "All services", exact: true })).toBeVisible();
    await expect(skeletonCards(page)).toHaveCount(8);

    release();

    await expect(skeletonCards(page)).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Pinned 1" })).toBeVisible();
  });

  test("the table view keeps its headers and loads with skeleton rows", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Table" }).click();
    await expect(page.getByRole("table", { name: "Services" })).toBeVisible();

    const release = await holdServices(page);
    await page.reload();

    const table = page.getByRole("table", { name: "Services" });
    await expect(page.getByRole("button", { name: "Table" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    await expect(table.getByRole("columnheader", { name: "Service" })).toBeVisible();
    await expect(table.locator('tbody tr[aria-hidden="true"]')).toHaveCount(8);

    release();

    await expect(table.locator('tbody tr[aria-hidden="true"]')).toHaveCount(0);
    await expect(table.getByRole("row", { name: /JupyterHub/ })).toBeVisible();
  });
});

test.describe("empty deployment", () => {
  test.use({ services: [[], { scope: "test" }] });

  test("explains that no services have been added", async ({ page, makeAxeBuilder }) => {
    await page.goto("/");

    await expect(page.getByRole("heading", { name: "No services yet" })).toBeVisible();
    await expect(
      page.getByText(/Your admin hasn't added any services to this deployment/),
    ).toBeVisible();
    await expect(page.getByRole("textbox", { name: "Search services" })).toHaveCount(0);

    expect((await makeAxeBuilder().analyze()).violations).toEqual([]);
  });
});

test("a search with no results explains itself and can be cleared", async ({
  page,
  makeAxeBuilder,
}) => {
  await page.goto("/");

  await page.getByRole("textbox", { name: "Search services" }).fill("kubeflow");

  await expect(page.getByRole("status")).toContainText('No services match "kubeflow"');
  await expect(page.getByRole("region", { name: /^All services/ })).toHaveCount(0);
  expect((await makeAxeBuilder().analyze()).violations).toEqual([]);

  await page.getByRole("button", { name: "Clear search" }).click();

  await expect(page.getByRole("textbox", { name: "Search services" })).toHaveValue("");
  await expect(page.getByRole("button", { name: "All services 5" })).toBeVisible();
});

test("a failed request shows an error that Try again recovers from", async ({
  page,
  makeAxeBuilder,
}) => {
  let failing = true;
  await page.route(SERVICES_URL, async (route) => {
    if (failing) {
      await route.fulfill({ status: 500, body: "boom" });
      return;
    }
    await route.fallback();
  });

  await page.goto("/");

  await expect(page.getByRole("alert")).toContainText("Couldn't load services");
  expect((await makeAxeBuilder().analyze()).violations).toEqual([]);

  failing = false;
  await page.getByRole("button", { name: "Try again" }).click();

  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Pinned 1" })).toBeVisible();
});
