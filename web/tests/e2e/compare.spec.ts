import { test, expect } from "@playwright/test";
import { signInAsDisposableUser, signInAsTestUser } from "./helpers";

// Dev seed (api/internal/seed/sql/dev/0004_compare_sample.sql): Liam and
// Noah are friends with overlapping Lego sets. In Lego Sets, both have
// Galaxy Explorer and Tree House; only Liam has Millennium Falcon and the
// Lamborghini; only Noah has Typewriter and NASA Apollo Saturn V; neither
// has AT-AT, Colosseum, or Titanic from the sample catalog.

test.describe("compare collections", () => {
  test("Liam compares with his friend Noah and sees all four buckets", async ({ page }) => {
    await page.goto("/");
    await signInAsTestUser(page, "Liam Johnson");
    await page.goto("/u/noah");
    await page.getByTestId("profile-compare-link").click();

    // The first category you both have opens, and the URL keeps it.
    await expect(page).toHaveURL(/\/u\/noah\/compare\?category=lego-sets$/);
    await expect(page.getByRole("heading", { name: "Compare with Noah Williams" })).toBeVisible();
    await expect(page.getByTestId("compare-count-both")).toHaveText("2");
    await expect(page.getByTestId("compare-count-mine_only")).toHaveText("2");
    await expect(page.getByTestId("compare-count-theirs_only")).toHaveText("2");
    await expect(page.getByTestId("compare-count-neither")).toHaveText("3");

    const rows = page.getByTestId("compare-row");
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0)).toContainText("Galaxy Explorer");
    await expect(rows.nth(1)).toContainText("Tree House");

    await page.getByRole("tab", { name: /Only you/ }).click();
    await expect(page).toHaveURL(/tab=mine_only/);
    await expect(rows.filter({ hasText: "Millennium Falcon" })).toHaveCount(1);
    await expect(rows.filter({ hasText: "Lamborghini" })).toHaveCount(1);

    await page.getByRole("tab", { name: /Only Noah/ }).click();
    await expect(rows.filter({ hasText: "Typewriter" })).toHaveCount(1);
    await expect(rows.filter({ hasText: "NASA Apollo Saturn V" })).toHaveCount(1);

    await page.getByRole("tab", { name: /Neither of you/ }).click();
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0)).toContainText("AT-AT");
    await expect(rows.nth(0).getByRole("button", { name: "Add to my collection" })).toBeVisible();

    // The state survives a reload.
    await page.reload();
    await expect(page.getByRole("tab", { name: /Neither of you/ })).toHaveAttribute("aria-selected", "true");
    await expect(rows).toHaveCount(3);
  });

  test("a collection page offers to compare in its category", async ({ page }) => {
    await page.goto("/");
    await signInAsTestUser(page, "Liam Johnson");
    await page.goto("/u/noah");
    await page.getByTestId("profile-collections").getByText("Lego Shelf").click();
    await page.getByTestId("collection-compare-link").click();
    await expect(page).toHaveURL(/\/u\/noah\/compare\?category=lego-sets$/);
    await expect(page.getByTestId("compare-count-both")).toHaveText("2");
  });

  test("add a catalog item neither of you has, straight from the comparison", async ({ page }) => {
    // A throwaway user, so the shared personas' collections never change.
    await signInAsDisposableUser(page);
    await page.goto("/u/liam/compare?category=lego-sets&tab=neither");
    const atat = page.getByTestId("compare-row").filter({ hasText: "AT-AT" });
    await atat.getByRole("button", { name: "Add to my collection" }).click();
    await page.getByTestId("add-to-collection-submit").click();
    await expect(page.getByTestId("compare-notice")).toContainText("Added AT-AT to Lego Sets.");
    await expect(page.getByTestId("compare-row").filter({ hasText: "AT-AT" })).toHaveCount(0);

    await page.getByRole("tab", { name: /Only you/ }).click();
    await expect(page.getByTestId("compare-row").filter({ hasText: "AT-AT" })).toHaveCount(1);
  });

  test("a stranger can't compare with a Friends-only profile", async ({ page }) => {
    // Olivia has no friends or followers in the dev seed.
    await page.goto("/");
    await signInAsTestUser(page, "Olivia Martinez");
    await page.goto("/u/noah");
    await expect(page.getByRole("heading", { name: "Noah Williams" })).toBeVisible();
    await expect(page.getByTestId("profile-compare-link")).toHaveCount(0);

    await page.goto("/u/noah/compare?category=lego-sets");
    await expect(page.getByTestId("compare-hidden")).toContainText("You can’t compare with this person");
    await expect(page.getByRole("tab")).toHaveCount(0);
  });
});
