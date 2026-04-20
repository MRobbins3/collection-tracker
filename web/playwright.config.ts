import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright is configured mobile-first: the default project is an iPhone 14
 * device profile, with Pixel 7 running alongside. Desktop is additive — we
 * only add it if and when desktop-specific UI ships.
 *
 * Running locally requires browser binaries (`pnpm exec playwright install
 * chromium webkit`). Docker-compose-based CI is wired up in Milestone 10.
 */
export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: process.env.E2E_BASE_URL || "http://localhost:3000",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "iphone-14", use: { ...devices["iPhone 14"] } },
    { name: "pixel-7", use: { ...devices["Pixel 7"] } },
  ],
});
