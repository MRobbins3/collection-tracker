import { test, expect, type Browser, type Page } from "@playwright/test";
import { API_URL, signInAsDisposableUser, signInAsTestUser, signOut } from "./helpers";

// Catalog suggestions and the admin moderation queue (ADR 0012). Olivia
// Martinez is the seeded admin (api/internal/seed/sql/dev/0001_dev_users.sql).
// Every run uses unique names in the Books category, whose catalog no other
// spec counts, so approved entries left behind by earlier runs don't matter.

function uniqueName(label: string): string {
  return `${label} ${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

function uniqueISBN(): string {
  return `978${Date.now().toString().slice(-10)}`;
}

async function createCollection(page: Page, categorySlug: string, name: string): Promise<string> {
  const res = await page.request.post(`${API_URL}/me/collections`, { data: { category_slug: categorySlug, name } });
  expect(res.ok()).toBe(true);
  return ((await res.json()) as { id: string }).id;
}

/** A second browser signed in as Olivia, the admin, alongside the submitter's. */
async function adminPage(browser: Browser): Promise<Page> {
  const context = await browser.newContext({
    baseURL: test.info().project.use.baseURL ?? process.env.E2E_BASE_URL ?? "http://localhost:3000",
  });
  const page = await context.newPage();
  await page.goto("/");
  await signInAsTestUser(page, "Olivia Martinez");
  return page;
}

/** Open a request from the queue (filtered to Books), paging until it shows. */
async function openFromQueue(page: Page, name: string, kind: "catalog_entry" | "catalog_correction") {
  await page.goto("/admin/moderation");
  await page.getByTestId("queue-kind").selectOption(kind);
  await page.getByTestId("queue-category").selectOption("books");
  const row = page.getByTestId("queue-item").filter({ hasText: name });
  await expect(page.getByTestId("queue-status")).toContainText("waiting for review");
  while (!(await row.isVisible())) {
    const more = page.getByTestId("queue-more");
    if (!(await more.isVisible())) break;
    await more.click();
  }
  await row.getByRole("link").click();
  await expect(page.getByTestId("review-name")).toHaveText(name);
}

async function searchBooksCatalog(page: Page, q: string) {
  await page.goto("/categories/books");
  await page.getByTestId("catalog-browser").getByTestId("catalog-search").fill(q);
}

test.describe("catalog moderation", () => {
  test("a user's suggestion is approved by the admin and appears in the public catalog", async ({ page }) => {
    const name = uniqueName("Moderated Book");
    await signInAsDisposableUser(page);
    const collectionId = await createCollection(page, "books", "My Shelf");
    await page.goto(`/my/${collectionId}`);

    // Add it from the add-item flow: not in the catalog, so suggest it.
    await page.getByTestId("new-item-search").fill(name);
    await page.getByTestId("new-item-suggest-open").click();
    await expect(page.getByTestId("new-item-suggest")).toBeChecked();
    await page.getByLabel("Author").fill("Test Author");
    await page.getByLabel("ISBN").fill(uniqueISBN());
    await page.getByTestId("item-form-submit").click();
    await expect(page.getByTestId("suggest-notice")).toContainText("An admin will review your suggestion");

    await page.goto("/my/submissions");
    const mine = page.getByTestId("submission").filter({ hasText: name });
    await expect(mine.getByTestId("submission-status")).toHaveText("Waiting for review");

    // Pending suggestions are never public.
    await searchBooksCatalog(page, name);
    await expect(page.getByTestId("catalog-status")).toHaveText("No matches");

    await signOut(page);
    await signInAsTestUser(page, "Olivia Martinez");
    await openFromQueue(page, name, "catalog_entry");
    await expect(page.getByTestId("review-submitted")).toContainText("Test Author");
    await page.getByTestId("review-approve").click();
    await expect(page.getByTestId("review-status")).toHaveText("Added to the catalog");
    await expect(page.getByTestId("review-outcome")).toContainText("by Olivia Martinez");

    await signOut(page);
    await searchBooksCatalog(page, name);
    await expect(page.getByTestId("catalog-status")).toHaveText("1 match");
    await page.getByTestId("catalog-browser").getByRole("link", { name: new RegExp(name) }).click();
    await expect(page.getByTestId("catalog-entry-name")).toHaveText(name);
  });

  test("a rejected suggestion shows the reviewer's reason to the submitter", async ({ page, browser }) => {
    const name = uniqueName("Rejected Book");
    await signInAsDisposableUser(page);
    const collectionId = await createCollection(page, "books", "My Shelf");
    await page.goto(`/my/${collectionId}`);
    await page.getByTestId("new-item-search").fill(name);
    await page.getByTestId("new-item-manual").click();
    await page.getByTestId("item-form-submit").click();

    // A free-form item: suggest it for the catalog from the item itself.
    const item = page.getByTestId("items-list").getByRole("listitem").filter({ hasText: name });
    await item.getByTestId("catalog-suggest").click();
    await expect(item.getByTestId("suggest-entry-name")).toHaveValue(name);
    await item.getByTestId("suggest-entry-submit").click();
    await expect(item.getByTestId("suggest-entry-result")).toContainText("An admin will review");

    const admin = await adminPage(browser);
    await openFromQueue(admin, name, "catalog_entry");
    await admin.getByTestId("review-reject").click();
    await admin.getByTestId("review-reject-reason").fill("Please add the author and ISBN from the cover.");
    await admin.getByTestId("review-reject-submit").click();
    await expect(admin.getByTestId("review-status")).toHaveText("Not accepted");
    await admin.context().close();

    await page.goto("/my/submissions");
    const mine = page.getByTestId("submission").filter({ hasText: name });
    await expect(mine.getByTestId("submission-status")).toHaveText("Not accepted");
    await expect(mine.getByTestId("submission-reason")).toContainText("Please add the author and ISBN from the cover.");
  });

  test("an approved correction updates the catalog entry", async ({ page, browser }) => {
    const name = uniqueName("Corrected Book");
    await signInAsDisposableUser(page);
    const admin = await adminPage(browser);

    // Setup through the API: a submitted, then approved, entry.
    const submitted = await page.request.post(`${API_URL}/me/catalog-submissions`, {
      data: { category_slug: "books", name, attributes: { author: "First Author", isbn: uniqueISBN(), edition: "First" } },
    });
    expect(submitted.status()).toBe(201);
    const { submission } = (await submitted.json()) as { submission: { id: string } };
    const approved = await admin.request.post(`${API_URL}/admin/moderation/${submission.id}/approve`, { data: {} });
    expect(approved.ok()).toBe(true);
    const { entry } = (await approved.json()) as { entry: { id: string } };

    // The submitter proposes a correction on the entry page.
    await page.goto(`/categories/books/catalog/${entry.id}`);
    await expect(page.getByTestId("catalog-entry-details")).toContainText("First");
    const correction = page.getByTestId("correction");
    await correction.getByTestId("correction-open").click();
    await correction.getByLabel("Edition").fill("Second");
    await correction.getByTestId("correction-note").fill("Checked the copyright page");
    await correction.getByTestId("correction-submit").click();
    await expect(correction.getByTestId("correction-sent")).toBeVisible();
    // Nothing changes until it's approved.
    await page.reload();
    await expect(page.getByTestId("catalog-entry-details")).not.toContainText("Second");

    await openFromQueue(admin, name, "catalog_correction");
    const diff = admin.getByTestId("review-diff-row");
    await expect(diff).toHaveCount(1);
    await expect(diff).toContainText("Edition");
    await expect(diff).toContainText("Second");
    await admin.getByTestId("review-approve").click();
    await expect(admin.getByTestId("review-status")).toHaveText("Correction applied");
    await admin.context().close();

    await page.reload();
    await expect(page.getByTestId("catalog-entry-details")).toContainText("Second");
    await page.goto("/my/submissions");
    await expect(
      page.getByTestId("submission").filter({ hasText: name }).first().getByTestId("submission-status"),
    ).toHaveText("Correction applied");
  });
});
