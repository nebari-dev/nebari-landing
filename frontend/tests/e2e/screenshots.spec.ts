/**
 * screenshots.spec.ts — intentional full-page screenshots for docs
 *
 * These tests are included in the chromium project and run whenever
 * CAPTURE_SCREENSHOTS=true is set (i.e. in the screenshots CI workflow).
 *
 * Each test saves a PNG directly to SCREENSHOT_DIR (default: ../docs/static/screenshots
 * relative to the frontend/ working directory) so the screenshots.yml workflow
 * can commit them without any path-flattening post-processing.
 *
 * The assertions are deliberately loose (page loads and header is visible)
 * so the screenshot job is not blocked by unrelated regressions in other
 * test files.
 */

import { fileURLToPath } from "node:url";
import path from "path";
import { expect, test } from "./fixtures/e2e";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Skip the whole file unless explicitly opted in, so normal CI runs are unaffected.
test.skip(
  !process.env.CAPTURE_SCREENSHOTS,
  "screenshots.spec.ts is skipped unless CAPTURE_SCREENSHOTS=true is set",
);

const screenshotDir =
  process.env.SCREENSHOT_DIR ?? path.resolve(__dirname, "../../../docs/static/screenshots");

test("homepage light theme", async ({ page }) => {
  // Pin light mode so the screenshot is deterministic regardless of OS preference.
  await page.addInitScript(() => {
    window.localStorage.setItem("launchpad:themeMode", "light");
  });

  await page.goto("/");
  await expect(page.locator("header")).toBeVisible();

  // Short pause so fonts and deferred renders settle before the screenshot.
  await page.waitForTimeout(500);

  await page.screenshot({
    fullPage: true,
    path: path.join(screenshotDir, "homepage-light.png"),
  });
});

test("homepage dark theme", async ({ page }) => {
  await page.addInitScript(() => {
    window.localStorage.setItem("launchpad:themeMode", "dark");
  });

  await page.goto("/");
  await expect(page.locator("header")).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/dark/);

  await page.waitForTimeout(500);

  await page.screenshot({
    fullPage: true,
    path: path.join(screenshotDir, "homepage-dark.png"),
  });
});

// Admin / user-management prototype (#208). Same light/dark pinning as above.
const ADMIN_SHOTS: { name: string; path: string; ready: RegExp }[] = [
  { name: "admin-overview", path: "/admin/overview", ready: /Overview/ },
  { name: "admin-users", path: "/admin/users", ready: /Users/ },
  { name: "admin-user-detail", path: "/admin/users/usr-alice", ready: /Alice Alvarez/ },
  { name: "admin-groups", path: "/admin/groups", ready: /Groups/ },
  { name: "admin-group-detail", path: "/admin/groups/grp-analysts", ready: /analysts/ },
  { name: "admin-roles", path: "/admin/roles", ready: /Roles/ },
  { name: "admin-services", path: "/admin/services", ready: /Service access/ },
  { name: "admin-service-detail", path: "/admin/services/svc-superset", ready: /Superset/ },
  { name: "admin-packs", path: "/admin/packs", ready: /Software packs/ },
  { name: "admin-pack-detail", path: "/admin/packs/data-science-pack", ready: /data-science-pack/ },
  { name: "admin-activity", path: "/admin/activity", ready: /Activity/ },
];

for (const mode of ["light", "dark"] as const) {
  for (const shot of ADMIN_SHOTS) {
    test(`${shot.name} ${mode} theme`, async ({ page }) => {
      await page.addInitScript((m) => {
        window.localStorage.setItem("launchpad:themeMode", m);
      }, mode);

      await page.goto(shot.path);
      await expect(page.getByRole("heading", { level: 2, name: shot.ready })).toBeVisible();
      await page.waitForTimeout(500);

      await page.screenshot({
        fullPage: true,
        path: path.join(screenshotDir, `${shot.name}-${mode}.png`),
      });
    });
  }
}
