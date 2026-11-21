import { expect, test } from "./axe-test";
import { simpleServices } from "./fixtures/services";

test("card Details opens the drawer without opening the service", async ({ page }) => {
  await page.goto("/");

  const popups: string[] = [];
  page.on("popup", (popup) => popups.push(popup.url()));

  const details = page.getByRole("button", { name: "Argo CD details" });
  await details.click();

  const drawer = page.getByRole("dialog", { name: "Argo CD" });
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveAccessibleDescription("Deploys apps from Git");
  await expect(drawer).toMatchAriaSnapshot(`
    - dialog "Argo CD":
      - heading "Argo CD"
      - button "Close"
      - paragraph: Deploys apps from Git
      - term: Category
      - definition: Platform
      - term: Version
      - definition: v2.13.0
      - term: Docs
      - definition:
        - link "argo-cd.readthedocs.io (opens in a new tab)":
          - /url: https://argo-cd.readthedocs.io/en/stable/
  `);
  expect(popups).toEqual([]);
});

test("drawer opens below the page header", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "Argo CD details" }).click();
  const drawer = page.getByRole("dialog", { name: "Argo CD" });
  await expect(drawer).toBeVisible();

  const header = await page.locator("header").first().boundingBox();
  const panel = await drawer.boundingBox();
  const viewport = page.viewportSize();
  expect(header).not.toBeNull();
  expect(panel).not.toBeNull();
  expect(Math.round(panel?.y ?? 0)).toBe(Math.round((header?.y ?? 0) + (header?.height ?? 0)));
  expect(Math.round((panel?.y ?? 0) + (panel?.height ?? 0))).toBe(viewport?.height);
});

test("drawer closes on Escape and returns focus to Details", async ({ page }) => {
  await page.goto("/");

  const details = page.getByRole("button", { name: "Argo CD details" });
  await details.click();

  const drawer = page.getByRole("dialog", { name: "Argo CD" });
  await expect(page.getByRole("button", { name: "Close" })).toBeFocused();

  await page.keyboard.press("Escape");
  await expect(drawer).toBeHidden();
  await expect(details).toBeFocused();
});

test("close button dismisses the drawer", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "Argo CD details" }).click();
  await page.getByRole("button", { name: "Close" }).click();

  await expect(page.getByRole("dialog")).toBeHidden();
});

test("table row Details opens the drawer instead of the service", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Table" }).click();

  const popups: string[] = [];
  page.on("popup", (popup) => popups.push(popup.url()));

  await page
    .getByRole("table", { name: "Services" })
    .getByRole("button", { name: "JupyterHub details" })
    .click();

  const drawer = page.getByRole("dialog", { name: "JupyterHub" });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("term")).toHaveText(["Category"]);
  expect(popups).toEqual([]);
});

test("open drawer has no automatically detectable WCAG A/AA violations", async ({
  page,
  makeAxeBuilder,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Argo CD details" }).click();
  await expect(page.getByRole("dialog", { name: "Argo CD" })).toBeVisible();
  await page.waitForFunction(() => document.getAnimations().length === 0);

  const results = await makeAxeBuilder().include('[data-slot="drawer-content"]').analyze();
  expect(results.violations).toEqual([]);
});

test.describe("simple layout", () => {
  test.use({ services: [simpleServices, { scope: "test" }] });

  test("Details opens the drawer", async ({ page }) => {
    await page.goto("/");

    await page.getByRole("button", { name: "Nebi details" }).click();

    await expect(page.getByRole("dialog", { name: "Nebi" })).toBeVisible();
  });
});
