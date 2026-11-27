import { test, expect, type Browser, type Page } from "@playwright/test";
import { API_URL, signInAsDisposableUser, signInAsTestUser } from "./helpers";

// User-built categories (ADR 0013): a disposable user builds a category from
// Lego and Vinyl fields, adds an option of their own, and uses it at once;
// nobody else sees it until Olivia Martinez (the seeded admin) approves it.
// Names are unique per run, so categories left by earlier runs don't matter.

function uniqueName(label: string): string {
  return `${label} ${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

async function otherBrowser(browser: Browser, persona?: string): Promise<Page> {
  const context = await browser.newContext({
    baseURL: test.info().project.use.baseURL ?? process.env.E2E_BASE_URL ?? "http://localhost:3000",
  });
  const page = await context.newPage();
  await page.goto("/");
  if (persona) await signInAsTestUser(page, persona);
  return page;
}

test.describe("user-built categories", () => {
  test("built from library fields, used privately at once, public after approval", async ({ page, browser }) => {
    const name = uniqueName("Brick Records");
    // A unique word, so options from categories approved by earlier runs never interfere.
    const word = Math.random().toString(36).replace(/[^a-z]/g, "").slice(0, 6) || "zork";
    const option = `Space ${word.charAt(0).toUpperCase()}${word.slice(1)}`;
    await signInAsDisposableUser(page);

    // Build it from the categories page.
    await page.goto("/categories");
    await page.getByTestId("create-category-link").click();
    await expect(page.getByRole("heading", { name: "Create a category" })).toBeVisible();
    await page.getByTestId("builder-name").fill(name);
    await page.getByTestId("builder-description").fill("Records that came with a brick set.");
    await page.getByTestId("builder-field-search").fill("theme");
    await page.getByTestId("builder-add-lego-sets-theme").click();
    await page.getByTestId("builder-field-search").fill("artist");
    await page.getByTestId("builder-add-vinyl-records-artist").click();

    // Turn Theme into a pick list with an option of our own, Title Cased.
    await page.getByTestId("builder-option-input-theme").fill(`space   ${word}`);
    await page.getByTestId("builder-option-suggestion-theme").click();
    await expect(page.getByTestId("builder-option-feedback-theme")).toHaveText(`Added “${option}”.`);
    await page.getByTestId("builder-option-input-theme").fill(`SPACE ${word}`);
    await page.getByTestId("builder-option-add-theme").click();
    await expect(page.getByTestId("builder-option-feedback-theme")).toHaveText(`“${option}” is already in the list.`);
    await page.getByTestId("builder-option-input-theme").fill("LEGO Ideas");
    await page.getByTestId("builder-option-add-theme").click();

    // The preview is the item form people will see.
    const preview = page.getByTestId("builder-preview");
    await expect(preview.getByTestId("attribute-field-theme")).toContainText(option);
    await expect(preview.getByTestId("attribute-field-artist")).toBeVisible();

    await page.getByTestId("builder-save").click();
    await expect(page.getByTestId("category-name")).toHaveText(name);
    await expect(page.getByTestId("category-status")).toHaveText("Private");
    const slug = new URL(page.url()).pathname.split("/").pop() ?? "";
    expect(slug).toMatch(/^brick-records-/);

    // Use it right away: a collection and an item with the new option.
    const res = await page.request.post(`${API_URL}/me/collections`, { data: { category_slug: slug, name: "Brick Vinyl" } });
    expect(res.ok()).toBe(true);
    const collectionId = ((await res.json()) as { id: string }).id;
    await page.goto(`/my/${collectionId}`);
    await page.getByTestId("new-item-search").fill("Pirate Shanties");
    await page.getByTestId("new-item-manual").click();
    await expect(page.getByTestId("new-item-suggest")).toHaveCount(0);
    await page.getByTestId("attribute-field-theme").selectOption(option);
    await page.getByTestId("attribute-field-artist").fill("The Minifigs");
    await page.getByTestId("item-form-submit").click();
    const item = page.getByTestId("items-list").getByText("Pirate Shanties");
    await expect(item).toBeVisible();
    await expect(page.getByTestId("items-list")).toContainText(option);

    // Another person can't see it: not in the list, not by its address.
    const liam = await otherBrowser(browser, "Liam Johnson");
    await liam.goto(`/categories/${slug}`);
    await expect(liam.getByTestId("category-error")).toBeVisible();
    const hidden = await liam.request.get(`${API_URL}/categories/${slug}`);
    expect(hidden.status()).toBe(404);
    await liam.goto("/categories");
    await expect(liam.getByTestId("categories-list")).toBeVisible();
    await expect(liam.getByTestId(`category-card-${slug}`)).toHaveCount(0);

    // Ask for the public list.
    await page.goto(`/categories/${slug}`);
    await page.getByTestId("category-ask-public").click();
    await page.getByTestId("category-ask-note").fill("Lots of people have these.");
    await page.getByTestId("category-ask-submit").click();
    await expect(page.getByTestId("category-status")).toHaveText("Waiting for review");

    // Olivia approves it from the queue.
    const admin = await otherBrowser(browser, "Olivia Martinez");
    await admin.goto("/admin/moderation");
    await admin.getByTestId("queue-kind").selectOption("category");
    const row = admin.getByTestId("queue-item").filter({ hasText: name });
    while (!(await row.isVisible())) {
      const more = admin.getByTestId("queue-more");
      if (!(await more.isVisible())) break;
      await more.click();
    }
    await row.getByRole("link").click();
    await expect(admin.getByTestId("review-name")).toHaveText(name);
    await expect(admin.getByTestId("review-category-fields")).toContainText(option);
    await admin.getByTestId("review-approve").click();
    await expect(admin.getByTestId("review-status")).toHaveText("On the public list");

    // Now everyone sees it.
    await liam.goto("/categories");
    await liam.getByTestId("category-search").fill(name);
    await expect(liam.getByTestId(`category-card-${slug}`)).toBeVisible();
    await expect(liam.getByTestId(`category-status-${slug}`)).toHaveCount(0);
    await page.goto(`/categories/${slug}`);
    await expect(page.getByTestId("category-status")).toHaveText("On the public list");

    await liam.context().close();
    await admin.context().close();
  });
});
