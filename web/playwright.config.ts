import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright is configured mobile-first: the default project is an iPhone 14
 * device profile, with Pixel 7 running alongside. Desktop is additive — we
 * only add it if and when desktop-specific UI ships.
 *
 * Running locally requires browser binaries (`bunx playwright install
 * chromium webkit`); see Documentation/testing.md.
 *
 * Signed-in specs use the local-only test-user sign-in (ADR 0008), so the
 * browser must reach the API at http://localhost:8080. Inside the web
 * container that address isn't the API, so docker-compose.yml sets
 * E2E_API_PROXY_TARGET and we start a small TCP forwarder for the run.
 */
const apiProxyTarget = process.env.E2E_API_PROXY_TARGET;

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
  webServer: apiProxyTarget
    ? {
        command: "node tests/e2e/api-proxy.mjs",
        url: "http://localhost:8080/healthz",
        reuseExistingServer: true,
      }
    : undefined,
  projects: [
    { name: "iphone-14", use: { ...devices["iPhone 14"] } },
    { name: "pixel-7", use: { ...devices["Pixel 7"] } },
  ],
});
