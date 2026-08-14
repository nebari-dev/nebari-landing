import { expect, test } from "./fixtures/e2e";

test("About opens a dialog with content, footer action, and close control", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: async (text: string) => {
          window.sessionStorage.setItem("copied-about-text", text);
        },
      },
    });
  });
  await page.goto("/");

  await page.getByRole("button", { name: /account menu/i }).click();
  const aboutItem = page.getByRole("menuitem", { name: /about/i });
  const signOutItem = page.getByRole("menuitem", { name: /sign out/i });

  for (const item of [aboutItem, signOutItem]) {
    await expect(item).toHaveCSS("width", "230px");
    await expect(item).toHaveCSS("padding", "4px 6px");
    await expect(item).toHaveCSS("gap", "8px");
    await expect(item).toHaveCSS("font-size", "14px");
    await expect(item).toHaveCSS("font-weight", "400");
    await expect(item).toHaveCSS("line-height", "20px");
    await expect(item.locator("svg")).toHaveCSS("width", "16px");
    await expect(item.locator("svg")).toHaveCSS("height", "16px");
    await expect(item.locator("svg")).toHaveCSS("flex-shrink", "0");
  }

  await aboutItem.click();

  const dialog = page.getByRole("dialog", { name: "About Nebari" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("Nebari Core")).toBeVisible();
  await expect(dialog.getByText("v0.1.0")).toBeVisible();
  await expect(dialog.getByText("a1b2c3d")).toBeVisible();
  await expect(dialog.getByText("Jun 28, 2026")).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Source" })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Software packs" })).toBeVisible();
  await expect(dialog.getByRole("heading", { name: "Services" })).toHaveCount(0);
  const repositoryLink = dialog.getByRole("link", {
    name: "nebari-infrastructure-core",
  });
  await expect(repositoryLink).toHaveAttribute(
    "href",
    "https://github.com/nebari-dev/nebari-infrastructure-core",
  );
  await expect(repositoryLink.locator("svg")).toBeVisible();
  await expect(dialog.getByText("Repository", { exact: true })).toHaveCount(0);
  const reportLink = dialog.getByRole("link", { name: "Report an issue" });
  const docsLink = dialog.getByRole("link", { name: "Documentation" });
  await expect(reportLink).toHaveAttribute(
    "href",
    "https://github.com/nebari-dev/nebari-infrastructure-core/issues/new",
  );
  await expect(docsLink).toHaveAttribute(
    "href",
    "https://github.com/nebari-dev/nebari-infrastructure-core",
  );
  const copyButton = dialog.getByRole("button", { name: "Copy all" });
  await expect(copyButton).toBeVisible();

  const reportBox = await reportLink.boundingBox();
  const docsBox = await docsLink.boundingBox();
  const copyBox = await copyButton.boundingBox();
  expect(reportBox?.x).toBeLessThan(docsBox?.x ?? 0);
  expect(docsBox?.x).toBeLessThan(copyBox?.x ?? 0);

  await copyButton.click();
  await expect(dialog.getByRole("button", { name: "Copied" })).toBeVisible();
  const copiedText = await page.evaluate(() => window.sessionStorage.getItem("copied-about-text"));
  expect(copiedText).toContain("Nebari Core");
  expect(copiedText).toContain("Version: v0.1.0");
  expect(copiedText).toContain("Commit: a1b2c3d");
  expect(copiedText).toContain("Last updated: Jun 28, 2026");
  expect(copiedText).toContain("SOURCE");
  expect(copiedText).toContain(
    "Repository: https://github.com/nebari-dev/nebari-infrastructure-core",
  );
  expect(copiedText).toContain("No software pack information available: —");
  expect(copiedText).not.toContain("SERVICES");

  const closeButton = dialog.getByRole("button", { name: /close/i });
  await expect(closeButton).toHaveCSS("width", "32px");
  await expect(closeButton).toHaveCSS("height", "32px");
  const titleBox = await dialog.getByRole("heading", { name: "About Nebari" }).boundingBox();
  const closeBox = await closeButton.boundingBox();
  const titleCenter = (titleBox?.y ?? 0) + (titleBox?.height ?? 0) / 2;
  const closeCenter = (closeBox?.y ?? 0) + (closeBox?.height ?? 0) / 2;
  expect(Math.abs(titleCenter - closeCenter)).toBeLessThanOrEqual(1);

  await closeButton.click();
  await expect(dialog).not.toBeVisible();
  await expect(page.getByRole("button", { name: /account menu/i })).toBeFocused();
});
