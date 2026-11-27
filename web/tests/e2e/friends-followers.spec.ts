import { test, expect, type Browser, type Page } from "@playwright/test";
import { API_URL, signInAsDisposableUser, uniqueHandle } from "./helpers";

// Every spec here changes relationships, so each uses fresh disposable users
// (dev auth only), never the seeded personas other specs rely on.

type Person = { page: Page; handle: string; close: () => Promise<void> };

/** A new disposable user in their own browser context, with a unique handle and the given visibility. */
async function newPerson(browser: Browser, visibility: string, prefix: string): Promise<Person> {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto("/");
  await signInAsDisposableUser(page);
  const handle = uniqueHandle(prefix);
  const res = await page.request.patch(`${API_URL}/me/profile`, {
    data: { handle, collection_visibility: visibility },
  });
  expect(res.ok()).toBe(true);
  return { page, handle, close: () => context.close() };
}

async function addCollection(p: Person, name: string) {
  const res = await p.page.request.post(`${API_URL}/me/collections`, { data: { category_slug: "coins", name } });
  expect(res.ok()).toBe(true);
}

test.describe("friends and followers", () => {
  test("a friend request, once accepted, unlocks a Friends-only collection", async ({ browser }) => {
    const asker = await newPerson(browser, "public", "ask");
    const owner = await newPerson(browser, "friends_only", "own");
    await addCollection(owner, "Pocket Change");

    await asker.page.goto(`/u/${owner.handle}`);
    await expect(asker.page.getByTestId("profile-collections-hidden")).toBeVisible();
    await asker.page.getByTestId("friend-add").click();
    await expect(asker.page.getByTestId("friend-status")).toHaveText("Friend request sent");
    await expect(asker.page.getByTestId("profile-collections-hidden")).toBeVisible();

    // The owner sees a badge in the header and accepts from the Requests tab.
    await owner.page.goto("/my");
    await expect(owner.page.getByTestId("nav-people-badge")).toContainText("1");
    await owner.page.getByTestId("nav-people").click();
    await owner.page.getByTestId("people-tab-requests").click();
    const incoming = owner.page.getByTestId("requests-friend-incoming");
    await expect(incoming).toContainText(`@${asker.handle}`);
    await incoming.getByTestId("friend-accept").click();
    await expect(incoming).toContainText("No friend requests.");
    await expect(owner.page.getByTestId("nav-people-badge")).toHaveCount(0);

    await asker.page.reload();
    await expect(asker.page.getByTestId("friend-status")).toContainText("Friends");
    await expect(asker.page.getByTestId("profile-collections").getByText("Pocket Change")).toBeVisible();

    // Both see each other in their Friends tab.
    await owner.page.getByTestId("people-tab-friends").click();
    await expect(owner.page.getByTestId("people-list-friends")).toContainText(`@${asker.handle}`);

    await asker.close();
    await owner.close();
  });

  test("a follow request can be denied, approved, and the follower removed again", async ({ browser }) => {
    const fan = await newPerson(browser, "public", "fan");
    const owner = await newPerson(browser, "followers_and_friends", "own");
    await addCollection(owner, "Silver Dollars");

    // Denied: nothing unlocks, and the fan can ask again.
    await fan.page.goto(`/u/${owner.handle}`);
    await fan.page.getByTestId("follow-request").click();
    await expect(fan.page.getByTestId("follow-status")).toHaveText("Follow requested");
    await owner.page.goto(`/u/${fan.handle}`);
    await expect(owner.page.getByTestId("follow-request-banner")).toBeVisible();
    await owner.page.getByTestId("follower-deny").click();
    await expect(owner.page.getByTestId("follow-request-banner")).toHaveCount(0);
    await fan.page.reload();
    await expect(fan.page.getByTestId("follow-request")).toBeVisible();
    await expect(fan.page.getByTestId("profile-collections-hidden")).toBeVisible();

    // Approved: the collection unlocks.
    await fan.page.getByTestId("follow-request").click();
    await expect(fan.page.getByTestId("follow-status")).toHaveText("Follow requested");
    await owner.page.goto("/people?tab=requests");
    const incoming = owner.page.getByTestId("requests-follow-incoming");
    await incoming.getByTestId("follower-approve").click();
    await expect(incoming).toContainText("No follow requests.");
    await fan.page.reload();
    await expect(fan.page.getByTestId("follow-status")).toContainText("Following");
    await expect(fan.page.getByTestId("profile-collections").getByText("Silver Dollars")).toBeVisible();

    // Removed: access goes away again.
    await owner.page.getByTestId("people-tab-followers").click();
    await owner.page.getByTestId("people-list-followers").getByTestId("follower-remove").click();
    await owner.page.getByTestId("confirm-yes").click();
    await expect(owner.page.getByTestId("people-empty-followers")).toBeVisible();
    await fan.page.reload();
    await expect(fan.page.getByTestId("follow-request")).toBeVisible();
    await expect(fan.page.getByTestId("profile-collections-hidden")).toBeVisible();

    await fan.close();
    await owner.close();
  });

  test("people search finds visible profiles and never a hidden one", async ({ browser }) => {
    const searcher = await newPerson(browser, "public", "srch");
    const hidden = await newPerson(browser, "private", "hide");
    const visible = await newPerson(browser, "friends_only", "seen");

    await searcher.page.goto("/people");
    const input = searcher.page.getByTestId("people-search-input");
    await input.fill(visible.handle);
    await expect(searcher.page.getByTestId("people-search-results")).toContainText(`@${visible.handle}`);

    await input.fill(hidden.handle);
    await expect(searcher.page.getByTestId("people-search-empty")).toBeVisible();
    await expect(searcher.page.getByText(`@${hidden.handle}`)).toHaveCount(0);

    // Seeded personas show up by name; Emma is Private and doesn't.
    await input.fill("Noah");
    await expect(searcher.page.getByTestId("people-search-results")).toContainText("Noah Williams");
    await input.fill("Emma Garcia");
    await expect(searcher.page.getByTestId("people-search-empty")).toBeVisible();

    await searcher.close();
    await hidden.close();
    await visible.close();
  });

  test("declining a Private person's request says it worked instead of 'not found'", async ({ browser }) => {
    const owner = await newPerson(browser, "public", "own");
    const asker = await newPerson(browser, "private", "priv");
    const sent = await asker.page.request.post(`${API_URL}/users/${owner.handle}/friend-request`);
    expect(sent.ok()).toBe(true);

    // Their request lets the owner see the Private card, to decide.
    await owner.page.goto(`/u/${asker.handle}`);
    await expect(owner.page.getByTestId("friend-request-banner")).toBeVisible();
    await expect(owner.page.getByTestId("profile-collections-hidden")).toBeVisible();
    await owner.page.getByTestId("friend-decline").click();
    await expect(owner.page.getByTestId("profile-gone")).toContainText("keeps their profile private");
    await expect(owner.page.getByTestId("profile-not-found")).toHaveCount(0);

    await owner.close();
    await asker.close();
  });
});
