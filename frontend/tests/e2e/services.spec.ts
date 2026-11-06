import { expect, test } from "./fixtures/e2e";
import { richerServices } from "./fixtures/services";

test("pinning moves a service between the Pinned and All services sections", async ({
  page,
}) => {
  await page.goto("/");

  const pinnedSection = page.getByRole("region", { name: /^Pinned/ });
  const allServices = page.getByRole("region", { name: /^All services/ });
  const jupyterInAll = allServices.locator('[data-slot="card"]', { hasText: "JupyterHub" });

  await pinnedSection.getByRole("button", { name: "Unpin service" }).click();

  await expect(page.getByRole("button", { name: /^Pinned/ })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "All services 6" })).toBeVisible();

  await jupyterInAll.getByRole("button", { name: "Pin service" }).click();

  await expect(page.getByRole("button", { name: "Pinned 1" })).toBeVisible();
  await expect(
    pinnedSection.getByRole("link", { name: "JupyterHub (opens in a new tab)" }),
  ).toBeVisible();
  await expect(jupyterInAll).toHaveCount(0);
});

test("unpinned cards reveal the pin control on hover and keyboard focus", async ({ page }) => {
  await page.goto("/");

  const allServices = page.getByRole("region", { name: /^All services/ });
  const grafanaCard = allServices.locator('[data-slot="card"]', { hasText: "Grafana" });
  const grafanaLink = allServices.getByRole("link", { name: "Grafana (opens in a new tab)" });
  const pinButton = grafanaCard.getByRole("button", { name: "Pin service" });
  const pinnedCard = page
    .getByRole("region", { name: /^Pinned/ })
    .locator('[data-slot="card"]', { hasText: "JupyterHub" });

  await expect(pinButton).toHaveCSS("opacity", "0");
  await expect(pinnedCard.getByRole("button", { name: "Unpin service" })).toHaveCSS(
    "opacity",
    "1",
  );

  // The card's full-size link sits on top, so that is what the pointer hovers.
  await grafanaLink.hover();
  await expect(pinButton).toHaveCSS("opacity", "1");

  await page.mouse.move(0, 0);
  await expect(pinButton).toHaveCSS("opacity", "0");

  await grafanaLink.focus();
  await page.keyboard.press("Tab");
  await expect(pinButton).toBeFocused();
  await expect(pinButton).toHaveCSS("opacity", "1");
});

test("every card and table row marks its name with a new-tab icon", async ({ page }) => {
  await page.goto("/");

  await expect(page.locator('[data-slot="card-title"] + svg.lucide-external-link')).toHaveCount(
    richerServices.length,
  );
  for (const service of richerServices) {
    await expect(
      page.getByRole("link", { name: `${service.name} (opens in a new tab)`, exact: true }),
    ).toHaveCount(1);
  }

  await page.getByRole("button", { name: "Table" }).click();
  const table = page.getByRole("table", { name: "Services" });

  await expect(table.locator("tbody a + svg.lucide-external-link")).toHaveCount(
    richerServices.length,
  );
  for (const service of richerServices) {
    await expect(
      table.getByRole("link", { name: `${service.name} (opens in a new tab)`, exact: true }),
    ).toBeVisible();
  }
});

test("unpinned table rows reveal the pin control on hover", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Table" }).click();

  const table = page.getByRole("table", { name: "Services" });
  const grafanaRow = table.getByRole("row", { name: /Grafana/ });
  const pinButton = grafanaRow.getByRole("button", { name: "Pin service" });

  await expect(pinButton).toHaveCSS("opacity", "0");
  await expect(
    table.getByRole("row", { name: /JupyterHub/ }).getByRole("button", { name: "Unpin service" }),
  ).toHaveCSS("opacity", "1");

  await grafanaRow.hover();
  await expect(pinButton).toHaveCSS("opacity", "1");
});

test.describe("services with several categories", () => {
  test.use({
    services: [
      richerServices.map((service) =>
        service.name === "MLflow"
          ? { ...service, category: ["Machine Learning", "Monitoring", "Platform"] }
          : service,
      ),
      { scope: "test" },
    ],
  });

  test("show the first category and a +N count on cards and rows", async ({ page }) => {
    await page.goto("/");

    const card = page
      .getByRole("region", { name: /^All services/ })
      .locator('[data-slot="card"]', { hasText: "MLflow" });
    await expect(card.getByText("Machine Learning")).toBeVisible();
    await expect(card.getByText("+2")).toBeVisible();
    await expect(card.getByText("Monitoring", { exact: true })).toHaveCount(0);

    await page.getByRole("button", { name: "Table" }).click();
    const row = page.getByRole("table", { name: "Services" }).getByRole("row", { name: /MLflow/ });
    await expect(row.getByText("+2")).toBeVisible();
  });
});
