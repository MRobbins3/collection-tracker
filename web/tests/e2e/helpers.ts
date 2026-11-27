import { expect, type Page } from "@playwright/test";

/** The API as the browser reaches it (compose and CI both publish it here). */
export const API_URL = process.env.E2E_API_URL || "http://localhost:8080";

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

/** Sign out from the header's account menu. */
export async function signOut(page: Page): Promise<void> {
  await page.getByTestId("account-menu-button").click();
  await page.getByTestId("logout-button").click();
}

/**
 * Create a brand-new throwaway user through the dev-only
 * POST /auth/dev/disposable and sign this browser context in as them. Use it
 * for specs that change or delete an account, so the shared seeded personas
 * other specs rely on are never modified. Returns the new user's handle.
 */
export async function signInAsDisposableUser(page: Page): Promise<{ handle: string; displayName: string }> {
  const res = await page.request.post(`${API_URL}/auth/dev/disposable`, { data: {} });
  expect(res.ok()).toBe(true);
  const { user } = (await res.json()) as { user: { handle: string; display_name: string } };
  return { handle: user.handle, displayName: user.display_name };
}

/** A handle no other run will pick: lowercase letters and digits, under 30 chars. */
export function uniqueHandle(prefix = "e2e"): string {
  const rand = Math.random().toString(36).slice(2, 6);
  return `${prefix}${Date.now().toString(36)}${rand}`.replace(/[^a-z0-9_]/g, "").slice(0, 30);
}
