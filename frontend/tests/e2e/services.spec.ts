import { expect, test } from "./fixtures/e2e";

test("pinning moves a service between the Pinned and All services sections", async ({
  page,
}) => {
  await page.goto("/");

  const pinnedSection = page.getByRole("region", { name: /^Pinned/ });
  const allServices = page.getByRole("region", { name: /^All services/ });
  const jupyterInAll = allServices.locator('[data-slot="card"]', { hasText: "JupyterHub" });

  await pinnedSection.getByRole("button", { name: "Unpin service" }).click();

  await expect(page.getByRole("button", { name: "Pinned 0" })).toBeVisible();
  await expect(page.getByText("No pinned services yet.")).toBeVisible();
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
