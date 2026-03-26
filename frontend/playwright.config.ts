import { defineConfig, devices } from "@playwright/test";
import dotenv from "dotenv";

dotenv.config({ path: ".env" });

const screenshotMode =
  (process.env.PW_SCREENSHOTS as "off" | "on" | "only-on-failure" | undefined) ??
  "off";

const outputDir = process.env.PW_OUTPUT_DIR ?? ".playwright/artifacts";
const realBaseURL = process.env.E2E_BASE_URL ?? "http://localhost:8080";
const mockBaseURL = process.env.MOCK_E2E_BASE_URL ?? "http://localhost:5173";

export default defineConfig({
  testDir: "./tests/e2e",
  outputDir,
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: [["html", { outputFolder: ".playwright/report", open: "never" }]],

  use: {
    headless: true,
    trace: "on-first-retry",
    screenshot: screenshotMode,
  },

  projects: [
    {
      name: "setup",
      testMatch: /.*\.setup\.ts/,
      use: {
        baseURL: realBaseURL,
      },
    },
    {
      name: "mock-chromium",
      testIgnore: /.*\.setup\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: mockBaseURL,
      },
    },
    {
      name: "chromium",
      dependencies: ["setup"],
      testIgnore: /.*\.setup\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: realBaseURL,
        storageState: ".playwright/auth/user.json",
      },
    },
  ],
});
