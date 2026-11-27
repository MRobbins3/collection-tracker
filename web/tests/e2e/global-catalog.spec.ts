import { test, expect, type Page } from "@playwright/test";
import { API_URL, signInAsDisposableUser } from "./helpers";

// The sample catalog is dev seed data (api/internal/seed/sql/dev/0002_sample_catalog.sql):
// eight Lego sets, including Millennium Falcon (75192), AT-AT (75313), and
// Galaxy Explorer (10497).

/** Create a collection through the API as the signed-in browser context. */
async function createCollection(page: Page, categorySlug: string, name: string): Promise<string> {
  const res = await page.request.post(`${API_URL}/me/collections`, { data: { category_slug: categorySlug, name } });
  expect(res.ok()).toBe(true);
  return ((await res.json()) as { id: string }).id;
}

test.describe("global catalog", () => {
  test("an anonymous visitor browses and searches a category's catalog", async ({ page }) => {
    await page.goto("/categories/lego-sets");
    const catalog = page.getByTestId("catalog-browser");
    await expect(catalog.getByTestId("catalog-entry")).toHaveCount(8);
    await expect(catalog.getByTestId("catalog-status")).toHaveText("8 entries");

    await catalog.getByTestId("catalog-search").fill("75192");
    await expect(catalog.getByTestId("catalog-entry")).toHaveCount(1);
    await expect(catalog.getByTestId("catalog-entry")).toContainText("Millennium Falcon");
    await expect(catalog.getByTestId("catalog-entry")).toContainText("Set number 75192");

    await catalog.getByRole("link", { name: /Millennium Falcon/ }).click();
    await expect(page).toHaveURL(/\/categories\/lego-sets\/catalog\/[0-9a-f-]+$/);
    await expect(page.getByTestId("catalog-entry-name")).toHaveText("Millennium Falcon");
    const details = page.getByTestId("catalog-entry-details");
    await expect(details).toContainText("Piece count");
    await expect(details).toContainText("7541");
    await expect(details).not.toContainText("piece_count");
    await expect(page.getByTestId("add-to-collection").getByTestId("signin-prompt")).toBeVisible();
  });

  test("a new user adds a catalog entry to a new collection and sees it linked", async ({ page }) => {
    await signInAsDisposableUser(page);
    await page.goto("/categories/lego-sets");
    await page.getByTestId("catalog-browser").getByRole("link", { name: /AT-AT/ }).click();
    await expect(page.getByTestId("catalog-entry-name")).toHaveText("AT-AT");

    const panel = page.getByTestId("add-to-collection");
    await expect(panel.getByTestId("add-to-collection-new")).toBeChecked();
    await panel.getByTestId("add-to-collection-name").fill("Star Wars Shelf");
    await panel.getByTestId("add-to-collection-submit").click();
    await expect(panel.getByTestId("add-to-collection-added")).toContainText("Added to Star Wars Shelf");
    await expect(panel.getByTestId("add-to-collection-owned")).toContainText("Star Wars Shelf");

    // Back on the category, the entry is marked as yours.
    await page.goto("/categories/lego-sets");
    const atat = page.getByTestId("catalog-entry").filter({ hasText: "AT-AT" });
    await expect(atat.getByTestId("catalog-entry-owned")).toBeVisible();

    // The new collection holds a linked item with the catalog details filled in.
    await page.goto("/my");
    await page.getByRole("link", { name: /Star Wars Shelf/ }).click();
    const item = page.getByTestId("items-list").getByRole("listitem").filter({ hasText: "AT-AT" });
    await expect(item).toContainText("75313");
    await expect(item.getByTestId("catalog-linked")).toBeVisible();
  });

  test("autocomplete links a new item, and a free-form item links to a suggested match", async ({ page }) => {
    await signInAsDisposableUser(page);
    const collectionId = await createCollection(page, "lego-sets", "Lego Shelf");
    await page.goto(`/my/${collectionId}`);

    // Autocomplete: pick a catalog match while adding.
    await page.getByTestId("new-item-search").fill("galaxy");
    await page.getByTestId("new-item-result").filter({ hasText: "Galaxy Explorer" }).click();
    await expect(page.getByTestId("new-item-linked")).toContainText("Galaxy Explorer");
    await expect(page.getByTestId("item-form-name")).toHaveValue("Galaxy Explorer");
    await page.getByTestId("item-form-submit").click();
    const galaxy = page.getByTestId("items-list").getByRole("listitem").filter({ hasText: "Galaxy Explorer" });
    await expect(galaxy.getByTestId("catalog-linked")).toBeVisible();

    // Free-form: add it manually, then link it to a suggestion.
    await page.getByTestId("new-item-search").fill("My Falcon");
    await page.getByTestId("new-item-manual").click();
    await page.getByLabel("Set number").fill("75192");
    await page.getByTestId("item-form-submit").click();
    const falcon = page.getByTestId("items-list").getByRole("listitem").filter({ hasText: "My Falcon" });
    await expect(falcon.getByTestId("catalog-linked")).toHaveCount(0);

    await falcon.getByRole("button", { name: "Find in catalog" }).click();
    const suggestion = falcon.getByTestId("catalog-suggestion").first();
    await expect(suggestion).toContainText("Millennium Falcon");
    await expect(suggestion).toContainText("Same set number");
    await suggestion.getByRole("button", { name: "Link to Millennium Falcon" }).click();
    await expect(falcon.getByTestId("catalog-linked")).toBeVisible();
    await expect(falcon).toContainText("My Falcon"); // the owner's name is kept

    // And unlink again.
    await falcon.getByRole("button", { name: "Unlink" }).click();
    await expect(falcon.getByRole("button", { name: "Find in catalog" })).toBeVisible();
  });
});
