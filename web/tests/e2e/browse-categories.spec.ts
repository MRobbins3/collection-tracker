import { test, expect } from "@playwright/test";

test.describe("anonymous category browse", () => {
  test("user can visit home, enter categories, and open a detail page", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /Track anything/i })).toBeVisible();

    await page.getByRole("link", { name: /Browse categories/i }).click();
    await expect(page).toHaveURL(/\/categories$/);

    const list = page.getByTestId("categories-list");
    await expect(list).toBeVisible();
    await expect(list.getByTestId("category-card-lego-sets")).toBeVisible();

    await list.getByTestId("category-card-lego-sets").click();
    await expect(page).toHaveURL(/\/categories\/lego-sets$/);
    await expect(page.getByTestId("category-name")).toHaveText("Lego Sets");
  });

  // The box is disabled until hydration (useHydrated). Before that fix,
  // fill() typed into the SSR markup, hydration wiped the text, and the
  // search request never fired. fill() waits for the box to be enabled.
  test("search narrows the list to matching categories", async ({ page }) => {
    await page.goto("/categories");
    await expect(page.getByTestId("category-card-lego-sets")).toBeVisible();
    const searched = page.waitForRequest((req) => new URL(req.url()).searchParams.get("q") === "vinyl");
    await page.getByTestId("category-search").fill("  vinyl ");
    await searched;
    await expect(page.getByTestId("category-card-vinyl-records")).toBeVisible();
    await expect(page.getByTestId("category-card-lego-sets")).toHaveCount(0);
  });

  test("an unknown slug renders the not-found state", async ({ page }) => {
    await page.goto("/categories/nope-not-real");
    await expect(page.getByTestId("category-error")).toBeVisible();
  });
});
