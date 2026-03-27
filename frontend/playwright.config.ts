import { defineConfig, devices } from "@playwright/test";
import dotenv from "dotenv";

dotenv.config({ path: ".env" });

const screenshotMode =
  (process.env.PW_SCREENSHOTS as "off" | "on" | "only-on-failure" | undefined) ??
  "off";

const outputDir = process.env.PW_OUTPUT_DIR ?? ".playwright/artifacts";
const E2E_BASE_URL = process.env.E2E_BASE_URL ?? "http://localhost:5173";

// When E2E_BASE_URL points to a real cluster (not the local Vite dev server),
// the production build ignores the __PW_E2E_AUTH__ mock, so we need a proper
// Keycloak login step before running any tests.
const isRealCluster = !!process.env.E2E_BASE_URL;

export default defineConfig({
  testDir: "./tests/e2e",
  outputDir,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: [["html", { outputFolder: ".playwright/report", open: "never" }]],

  use: {
    baseURL: E2E_BASE_URL,
    headless: true,
    trace: "on-first-retry",
    screenshot: screenshotMode,
    serviceWorkers: "block",
  },

  projects: [
    // Auth setup — runs only when targeting a real cluster so Keycloak
    // credentials are stored in .playwright/auth/user.json before the
    // actual test projects start.
    ...(isRealCluster
      ? [
          {
            name: "auth-setup",
            testMatch: /auth\.setup\.ts/,
          },
        ]
      : []),

    {
      name: "chromium",
      use: {
        ...devices["Desktop Chrome"],
        // Reuse the Keycloak session captured by auth-setup when running
        // against a real cluster; skip it for local dev (mock auth is used).
        ...(isRealCluster && {
          storageState: ".playwright/auth/user.json",
        }),
      },
      dependencies: isRealCluster ? ["auth-setup"] : [],
    },
  ],

  webServer: {
    command: "npm run dev",
    url: E2E_BASE_URL,
    // Reuse an already-running server when:
    //   - not in CI (local dev), OR
    //   - E2E_BASE_URL is explicitly set (e.g. screenshots workflow pointing at
    //     a port-forwarded cluster — no dev server to start).
    reuseExistingServer: !process.env.CI || !!process.env.E2E_BASE_URL,
  },
});
