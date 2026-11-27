import { test, expect } from "@playwright/test";
import { signInAsTestUser } from "./helpers";

test.describe("test-user sign-in", () => {
  test("sign in as a persona, see their collections, sign out", async ({ page }) => {
    await page.goto("/");
    await signInAsTestUser(page, "Liam Johnson");

    await page.getByTestId("nav-my-collections").click();
    await expect(page).toHaveURL(/\/my$/);
    await expect(page.getByText("Hey Liam Johnson")).toBeVisible();
    const list = page.getByTestId("my-collections-list");
    await expect(list.getByText("Lego Display Shelf")).toBeVisible();
    await expect(list.getByText("Classic Rock Vinyl")).toBeVisible();

    // The session survives a full reload (the cookie, not client state).
    // A cold reload re-hydrates the dev server bundle, so allow extra time.
    await page.reload();
    await expect(page.getByTestId("auth-menu-signed-in")).toContainText("Liam Johnson", { timeout: 15_000 });
    await expect(page.getByTestId("my-collections-list").getByText("Lego Display Shelf")).toBeVisible();

    await page.getByTestId("logout-button").click();
    await expect(page.getByTestId("signin-prompt")).toBeVisible();
    await expect(page.getByRole("button", { name: "Test sign-in" })).toBeVisible();
    await expect(page.getByTestId("my-collections-list")).toHaveCount(0);

    // Still anonymous after a reload: the cookie was cleared server-side.
    await page.reload();
    await expect(page.getByRole("button", { name: "Test sign-in" })).toBeVisible({ timeout: 15_000 });
    await expect(page.getByTestId("signin-prompt")).toBeVisible();
  });

  test("the sign-in prompt on My collections offers test users too", async ({ page }) => {
    await page.goto("/my");
    const prompt = page.getByTestId("signin-prompt");
    await prompt.getByRole("button", { name: "Sign in as a test user" }).click();
    await expect(prompt.getByText(/local testing only/i)).toBeVisible();
    await prompt.getByRole("button", { name: /Emma Garcia/ }).click();
    await expect(page.getByText("Hey Emma Garcia")).toBeVisible();
    await expect(page.getByTestId("new-collection-form")).toBeVisible();
  });
});

test.describe("anonymous visitors to signed-in pages", () => {
  // CLAUDE.md: no red 401s in the console at app boot. Signed-in data
  // fetches must wait for /me instead of firing and failing.
  for (const path of ["/my", "/my/d0000000-0000-4000-8000-000000000101"]) {
    test(`${path} makes no /me/collections request`, async ({ page }) => {
      const meRequests: string[] = [];
      page.on("request", (req) => {
        if (new URL(req.url()).pathname.startsWith("/me/collections")) meRequests.push(req.url());
      });
      await page.goto(path);
      // The test-user button appears once auth has loaded after hydration.
      await expect(page.getByRole("button", { name: "Test sign-in" })).toBeVisible();
      await expect(page.getByTestId("signin-prompt")).toBeVisible();
      await page.waitForLoadState("networkidle");
      expect(meRequests).toEqual([]);
    });
  }
});

test.describe("signed-in collection flow", () => {
  test("create a collection, add an item, and delete the collection", async ({ page }) => {
    // Unique name: projects run in parallel against one shared database.
    const name = `E2E Coins ${test.info().project.name} ${Date.now()}`;
    page.on("dialog", (d) => void d.accept());

    await page.goto("/my");
    await signInAsTestUser(page, "Noah Williams");

    const form = page.getByTestId("new-collection-form");
    await form.getByTestId("new-collection-category").selectOption({ label: "Coins" });
    await form.getByTestId("new-collection-name").fill(name);
    await form.getByTestId("new-collection-submit").click();

    await page.getByRole("link", { name }).click();
    await expect(page.getByRole("heading", { name })).toBeVisible();
    await expect(page.getByTestId("items-empty")).toBeVisible();

    await page.getByTestId("new-item-search").fill("Buffalo Nickel");
    await page.getByTestId("new-item-manual").click();
    await page.getByTestId("item-form-submit").click();
    await expect(page.getByTestId("items-list").getByText("Buffalo Nickel")).toBeVisible();

    await page.getByText("Collection settings").click();
    await page.getByTestId("delete-collection").click();
    await expect(page).toHaveURL(/\/my$/);
    await expect(page.getByRole("link", { name })).toHaveCount(0);
  });
});
