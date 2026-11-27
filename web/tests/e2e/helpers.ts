import { expect, type Page } from "@playwright/test";

/**
 * Sign in through the local-only test-user picker in the header (ADR 0008).
 * Requires the API to run with DEV_AUTH_ENABLED=true and SEED_DEV_DATA=true,
 * as docker compose and the CI e2e job do. `displayName` is a seeded persona
 * from api/internal/seed/sql/dev/0001_dev_users.sql.
 */
export async function signInAsTestUser(page: Page, displayName: string): Promise<void> {
  // The button appears after hydration, once /auth/providers has loaded.
  await page.getByRole("button", { name: "Test sign-in" }).click();
  await page.getByRole("button", { name: new RegExp(displayName) }).click();
  await expect(page.getByTestId("auth-menu-signed-in")).toContainText(displayName);
}
