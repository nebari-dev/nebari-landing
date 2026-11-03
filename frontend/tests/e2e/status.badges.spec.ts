import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures/e2e";

function card(page: Page, name: string) {
  return page.locator('[data-slot="card"]', { hasText: name });
}

function backgroundOf(locator: Locator) {
  return locator.evaluate((element) => getComputedStyle(element).backgroundColor);
}

test.describe("regular user", () => {
  test("a down service is unavailable and a degraded one shows no badge", async ({ page }) => {
    await page.goto("/");

    const dask = card(page, "Dask Gateway");
    await expect(dask.getByText("Unavailable", { exact: true })).toBeVisible();
    await expect(dask.locator("svg.lucide-external-link")).toHaveCount(0);
    await expect(card(page, "MLflow").getByText(/^(Degraded|Down|Unavailable)$/)).toHaveCount(0);
    await expect(page.getByText(/^(Degraded|Down)$/)).toHaveCount(0);

    await page.getByRole("button", { name: "Table" }).click();
    const table = page.getByRole("table", { name: "Services" });

    await expect(
      table.getByRole("row", { name: /Dask Gateway/ }).getByText("Unavailable", { exact: true }),
    ).toBeVisible();
    await expect(table.getByText(/^(Degraded|Down)$/)).toHaveCount(0);
  });
});

test.describe("admin", () => {
  test.use({ isAdmin: true });

  test("degraded and down services show their badge on the card and the table row", async ({
    page,
  }) => {
    await page.goto("/");

    await expect(card(page, "MLflow").getByText("Degraded", { exact: true })).toBeVisible();
    const dask = card(page, "Dask Gateway");
    await expect(dask.getByText("Down", { exact: true })).toBeVisible();
    await expect(dask.locator("svg.lucide-external-link")).toHaveCount(1);
    await expect(page.getByText("Unavailable", { exact: true })).toHaveCount(0);
    await expect(card(page, "Grafana").getByText(/^(Degraded|Down)$/)).toHaveCount(0);

    await page.getByRole("button", { name: "Table" }).click();
    const table = page.getByRole("table", { name: "Services" });

    await expect(
      table.getByRole("row", { name: /MLflow/ }).getByText("Degraded", { exact: true }),
    ).toBeVisible();
    await expect(
      table.getByRole("row", { name: /Dask Gateway/ }).getByText("Down", { exact: true }),
    ).toBeVisible();
  });

  for (const theme of ["light", "dark"] as const) {
    test(`degraded and down badges use distinct theme colors in ${theme} mode`, async ({
      page,
    }) => {
      await page.addInitScript((mode) => {
        window.localStorage.setItem("launchpad:themeMode", mode);
      }, theme);
      await page.goto("/");

      const degraded = card(page, "MLflow").getByText("Degraded", { exact: true });
      const down = card(page, "Dask Gateway").getByText("Down", { exact: true });
      await expect(degraded).toBeVisible();
      await expect(down).toBeVisible();

      const tokens = await page.evaluate(() => {
        const probe = document.createElement("span");
        document.body.append(probe);
        const resolve = (token: string) => {
          probe.style.backgroundColor = `var(${token})`;
          return getComputedStyle(probe).backgroundColor;
        };
        const colors = { warning: resolve("--warning"), destructive: resolve("--destructive") };
        probe.remove();
        return colors;
      });

      await expect.poll(() => backgroundOf(degraded)).toBe(tokens.warning);
      await expect.poll(() => backgroundOf(down)).toBe(tokens.destructive);
      expect(tokens.warning).not.toBe(tokens.destructive);
    });
  }
});
