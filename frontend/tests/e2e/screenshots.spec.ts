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

import path from "path";
import { test, expect } from "./fixtures/e2e"
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Skip the whole file unless explicitly opted in, so normal CI runs are unaffected.
test.skip(
    !process.env.CAPTURE_SCREENSHOTS,
    "screenshots.spec.ts is skipped unless CAPTURE_SCREENSHOTS=true is set",
);

const screenshotDir =
    process.env.SCREENSHOT_DIR ??
    path.resolve(__dirname, "../../../docs/static/screenshots");

test("homepage light theme", async ({ page }) => {
    // Light mode is the default; clear any persisted dark-mode preference.
    await page.addInitScript(() => {
        window.localStorage.removeItem("launchpad:isDarkMode");
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
        window.localStorage.setItem("launchpad:isDarkMode", "true");
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
