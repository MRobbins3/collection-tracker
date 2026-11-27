import { test, expect } from "@playwright/test";
import { signInAsDisposableUser, signInAsTestUser, signOut, uniqueHandle } from "./helpers";

// Specs that edit or delete an account use a disposable user (dev auth only),
// never a seeded persona, because specs run in parallel on one database.

test.describe("profile and privacy settings", () => {
  test("edit your profile, then another user sees it when Public and not when Private", async ({ page, browser }) => {
    await signInAsDisposableUser(page);
    await page.goto("/settings");

    const handle = uniqueHandle();
    const name = page.getByTestId("profile-display-name");
    await name.fill("casey tester");
    // Title Case is suggested, not forced.
    await page.getByTestId("display-name-suggestion").click();
    await expect(name).toHaveValue("Casey Tester");

    await page.getByTestId("profile-handle").fill(handle);
    await expect(page.getByTestId("handle-status")).toContainText(`@${handle} is available`);
    await page.getByTestId("visibility-public").check();
    await page.getByTestId("profile-save").click();
    await expect(page.getByTestId("profile-saved")).toBeVisible();
    await expect(page.getByTestId("auth-menu-signed-in")).toContainText("Casey Tester");

    // The header menu links to the new profile.
    await page.getByTestId("account-menu-button").click();
    await page.getByTestId("account-menu-profile").click();
    await expect(page).toHaveURL(new RegExp(`/u/${handle}$`));
    await expect(page.getByRole("heading", { name: "Casey Tester" })).toBeVisible();
    await expect(page.getByTestId("profile-edit-link")).toBeVisible();

    // Someone else, signed in as a seeded persona, in a separate browser context.
    const other = await browser.newContext();
    const liam = await other.newPage();
    await liam.goto("/");
    await signInAsTestUser(liam, "Liam Johnson");
    await liam.goto(`/u/${handle}`);
    await expect(liam.getByRole("heading", { name: "Casey Tester" })).toBeVisible();
    await expect(liam.getByTestId("profile-collections-empty")).toBeVisible();
    await expect(liam.getByTestId("profile-edit-link")).toHaveCount(0);

    // Now go Private: the profile disappears for everyone else.
    await page.goto("/settings");
    await page.getByTestId("visibility-private").check();
    await page.getByTestId("profile-save").click();
    await expect(page.getByTestId("profile-saved")).toBeVisible();

    await liam.reload();
    await expect(liam.getByTestId("profile-not-found")).toBeVisible({ timeout: 15_000 });
    await expect(liam.getByText("Casey Tester")).toHaveCount(0);

    // The owner still sees it.
    await page.goto(`/u/${handle}`);
    await expect(page.getByRole("heading", { name: "Casey Tester" })).toBeVisible();
    await expect(page.getByTestId("profile-visibility-note")).toContainText("Only you");
    await other.close();
  });

  test("a taken handle is flagged before saving", async ({ page }) => {
    await signInAsDisposableUser(page);
    await page.goto("/settings");
    await page.getByTestId("profile-handle").fill("liam");
    await expect(page.getByTestId("handle-status")).toContainText("That handle is taken");
  });
});

test.describe("viewing other people's profiles", () => {
  test("a Public profile shows its collections read-only; a Private one is not found", async ({ page }) => {
    // Anonymous visitors can see a Public collector.
    await page.goto("/u/liam");
    await expect(page.getByRole("heading", { name: "Liam Johnson" })).toBeVisible();
    const list = page.getByTestId("profile-collections");
    await list.getByText("Lego Display Shelf").click();
    await expect(page.getByRole("heading", { name: "Lego Display Shelf" })).toBeVisible();
    const items = page.getByTestId("public-items-list");
    await expect(items.getByText("Millennium Falcon")).toBeVisible();
    await expect(items.getByRole("button")).toHaveCount(0);

    // Emma's collections are Private, so her profile is hidden from Liam.
    await page.goto("/");
    await signInAsTestUser(page, "Liam Johnson");
    await page.goto("/u/emma");
    await expect(page.getByTestId("profile-not-found")).toBeVisible();

    // Noah shares with friends only: his card shows, his collections don't.
    await page.goto("/u/noah");
    await expect(page.getByRole("heading", { name: "Noah Williams" })).toBeVisible();
    await expect(page.getByTestId("profile-collections-hidden")).toContainText("friends");
  });
});

test.describe("deleting your account", () => {
  test("type your handle to delete the account; you're signed out and the profile is gone", async ({ page }) => {
    const { handle } = await signInAsDisposableUser(page);
    await page.goto("/settings");

    await page.getByTestId("delete-account-start").click();
    const submit = page.getByTestId("delete-account-submit");
    await expect(submit).toBeDisabled();
    await page.getByTestId("delete-account-confirm").fill(`@${handle}`);
    await submit.click();

    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByRole("button", { name: "Test sign-in" })).toBeVisible();
    await expect(page.getByTestId("auth-menu-signed-in")).toHaveCount(0);

    await page.goto(`/u/${handle}`);
    await expect(page.getByTestId("profile-not-found")).toBeVisible();
  });
});

test("sign out from the account menu", async ({ page }) => {
  await signInAsDisposableUser(page);
  await page.goto("/");
  await expect(page.getByTestId("auth-menu-signed-in")).toContainText("Disposable Tester");
  await signOut(page);
  await expect(page.getByRole("button", { name: "Test sign-in" })).toBeVisible();
});
