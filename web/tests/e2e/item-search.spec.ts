import { test, expect } from "@playwright/test";
import { signInAsTestUser } from "./helpers";

// Liam Johnson's seeded collections (api/internal/seed/sql/dev/0001_dev_users.sql)
// include the album Abbey Road (artist: The Beatles) in "Classic Rock Vinyl"
// and the Lego set Millennium Falcon in "Lego Display Shelf".
test.describe("search your own items", () => {
  test("find an item by an attribute value, open its collection, and come back", async ({ page }) => {
    await page.goto("/my");
    await signInAsTestUser(page, "Liam Johnson");
    await expect(page.getByTestId("my-collections-list")).toBeVisible();

    await page.getByLabel("Search your items").fill("  the   beatles ");
    const result = page.getByTestId("item-search-result").filter({ hasText: "Abbey Road" });
    await expect(result).toContainText("Classic Rock Vinyl · Vinyl Records");
    await expect(result).toContainText("Artist: The Beatles");
    await expect(page.getByTestId("item-search-result")).toHaveCount(1);
    // Search replaces the collections list while it's active.
    await expect(page.getByTestId("my-collections-list")).toHaveCount(0);
    await expect(page).toHaveURL(/\/my\?q=the\+beatles$/);

    await result.getByRole("link").click();
    await expect(page.getByRole("heading", { name: "Classic Rock Vinyl" })).toBeVisible();
    await expect(page.getByTestId("items-list").getByRole("heading", { name: "Abbey Road" })).toBeVisible();

    await page.goBack();
    await expect(page.getByLabel("Search your items")).toHaveValue("the beatles");
    await expect(page.getByTestId("item-search-result").filter({ hasText: "Abbey Road" })).toBeVisible();

    await page.getByLabel("Search your items").fill("");
    await expect(page.getByTestId("my-collections-list")).toBeVisible();
    await expect(page).toHaveURL(/\/my$/);
  });

  test("shows an empty state when nothing matches", async ({ page }) => {
    await page.goto("/my");
    await signInAsTestUser(page, "Liam Johnson");
    await page.getByLabel("Search your items").fill("zebra crossing");
    await expect(page.getByTestId("item-search-empty")).toContainText("“zebra crossing”");
  });

  test("never shows another user's items", async ({ page }) => {
    await page.goto("/my");
    await signInAsTestUser(page, "Noah Williams");
    await page.getByLabel("Search your items").fill("millennium falcon");
    await expect(page.getByTestId("item-search-empty")).toBeVisible();
    await expect(page.getByTestId("item-search-result")).toHaveCount(0);
  });
});
