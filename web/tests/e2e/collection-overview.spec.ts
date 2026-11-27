import { test, expect, type Page } from "@playwright/test";
import { signInAsDisposableUser, signInAsTestUser } from "./helpers";

// Liam Johnson's seeded items (api/internal/seed/sql/dev/0001_dev_users.sql),
// newest first by their seeded dates: Rumours, Lamborghini Sián FKP 37, Tree
// House, Galaxy Explorer, Abbey Road, Millennium Falcon. Rumours and Abbey
// Road are the vinyl records; the rest are Lego sets.
function itemNames(page: Page) {
  return page.getByTestId("overview-item").getByRole("heading");
}

test.describe("collection overview", () => {
  test("a collector sorts and filters everything they've added, and the URL keeps up", async ({ page }) => {
    await page.goto("/my");
    await signInAsTestUser(page, "Liam Johnson");

    // Collections view groups collections under their category.
    const list = page.getByTestId("my-collections-list");
    await expect(list.getByRole("heading", { level: 2 })).toHaveText(["Lego Sets", "Vinyl Records"]);
    await expect(page.getByTestId("category-group-lego-sets")).toContainText("Lego Display Shelf");

    await page.getByRole("button", { name: "All items" }).click();
    await expect(page).toHaveURL(/\/my\?view=items$/);
    await expect(page.getByTestId("overview-item-count")).toHaveText("6 items");
    await expect(itemNames(page).first()).toHaveText("Rumours");
    await expect(itemNames(page).last()).toHaveText("Millennium Falcon");

    await page.getByLabel("Sort by").selectOption({ label: "Name (A–Z)" });
    await expect(page).toHaveURL(/sort=name_asc/);
    await expect(itemNames(page)).toHaveText([
      "Abbey Road",
      "Galaxy Explorer",
      "Lamborghini Sián FKP 37",
      "Millennium Falcon",
      "Rumours",
      "Tree House",
    ]);

    const filters = page.getByRole("group", { name: "Filter by category" });
    await filters.getByRole("button", { name: /Vinyl Records/ }).click();
    await expect(page).toHaveURL(/category=vinyl-records/);
    await expect(itemNames(page)).toHaveText(["Abbey Road", "Rumours"]);
    await expect(filters.getByRole("button", { name: /Vinyl Records/ })).toHaveAttribute("aria-pressed", "true");

    // A reload (or a shared link) restores the same view.
    await page.reload();
    await expect(itemNames(page)).toHaveText(["Abbey Road", "Rumours"]);
    await expect(page.getByLabel("Sort by")).toHaveValue("name_asc");

    // Back undoes the filter.
    await page.goBack();
    await expect(page).not.toHaveURL(/category=/);
    await expect(itemNames(page)).toHaveCount(6);

    await page.getByLabel("Sort by").selectOption({ label: "Name (Z–A)" });
    await expect(itemNames(page).first()).toHaveText("Tree House");

    // Filters apply to the collections view too.
    await page.getByRole("button", { name: "Collections" }).click();
    await filters.getByRole("button", { name: /Lego Sets/ }).click();
    await expect(list.getByRole("heading", { level: 2 })).toHaveText(["Lego Sets"]);
  });

  test("a new user adds an empty category and shows or hides empty categories", async ({ page }) => {
    await signInAsDisposableUser(page);
    await page.goto("/my");
    await expect(page.getByTestId("my-collections-empty")).toBeVisible();

    await page.getByRole("button", { name: "Add a category" }).click();
    const panel = page.getByTestId("add-category-panel");
    await panel.getByRole("button", { name: "Add Coins to my collection" }).click();
    await expect(panel.getByRole("button", { name: "Remove Coins from my collection" })).toBeVisible();
    await panel.getByRole("button", { name: "Close" }).click();

    const coins = page.getByTestId("category-group-coins");
    await expect(coins).toContainText("Nothing added yet");
    await expect(coins.getByRole("button", { name: "Start a collection" })).toBeVisible();

    const toggle = page.getByRole("switch", { name: "Show empty categories" });
    await expect(toggle).toHaveAttribute("aria-checked", "true");
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-checked", "false");
    await expect(coins).toHaveCount(0);
    await expect(page.getByTestId("hidden-empty-note")).toContainText("1 empty category hidden");

    // The setting is saved to the account, not just the page.
    await page.reload();
    await expect(page.getByRole("switch", { name: "Show empty categories" })).toHaveAttribute("aria-checked", "false");
    await expect(page.getByTestId("category-group-coins")).toHaveCount(0);

    await page.getByTestId("hidden-empty-note").getByRole("button", { name: "Show it" }).click();
    await expect(page.getByTestId("category-group-coins")).toBeVisible();
    await expect(page.getByRole("switch", { name: "Show empty categories" })).toHaveAttribute("aria-checked", "true");

    await page.getByRole("button", { name: "Remove Coins from my collection" }).click();
    await expect(page.getByTestId("my-collections-empty")).toBeVisible();
  });

  test("add a category from the category list, then start a collection in it", async ({ page }) => {
    await signInAsDisposableUser(page);
    await page.goto("/categories");
    await page.getByRole("button", { name: "Add Stamps to my collection" }).click();
    await expect(page.getByRole("button", { name: "Remove Stamps from my collection" })).toBeVisible();

    await page.goto("/my");
    const stamps = page.getByTestId("category-group-stamps");
    await expect(stamps).toContainText("Nothing added yet");
    await stamps.getByRole("button", { name: "Start a collection" }).click();
    await expect(page.getByRole("heading", { name: "Stamps", level: 1 })).toBeVisible();

    await page.goto("/categories/stamps");
    await expect(page.getByTestId("category-pin-panel")).toContainText("You collect Stamps.");
    await expect(page.getByRole("link", { name: "In my collection" })).toHaveAttribute("href", "/my?category=stamps");
  });
});
