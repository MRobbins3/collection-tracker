# Review notes: collection-overview

Decisions made without the owner, for review. Covers the collection overview, category pins, and the show-empty setting (ADR 0010, Proposed).

## Data model

- **Adding a category is a pin (`user_pinned_categories`), not an empty collection.** Why: collections are user-named and show on the public profile; a pin is a private view preference with no name, no items, and no catalog link, so it can't conflict with ADR 0004 or user-built categories. Alternative: create an empty collection named after the category (one fewer table, but leaks to profiles and clutters collection lists).
- **Show/hide empty is one per-user boolean on `users` (`show_empty_categories`, default on).** Why: the request was a single toggle; default on so a just-added category is visible. Alternative: a `user_category_preferences(pinned, hidden)` table with per-category hiding. A column can be added if per-category hiding is wanted.
- **"Empty" means zero items**, so a category whose collections are all empty is hidden too. The overview shows "N empty categories hidden. Show them" so a newly created empty collection never silently vanishes. Alternative: "empty" = no collections (pinned only).
- **Counts are item rows, not summed quantities**, matching the existing collection `item_count` (3 Kennedy half dollars in one row count as 1).
- **Pins are allowed on every category** because every category is global and approved today. The user-built categories PR must restrict this (noted in `next-steps.md` and in `store.Overview.Pin`).

## API

- **One endpoint, `GET /me/overview`**, returns categories, the setting, and a page of items, so the page loads in one request. `limit=0` returns categories only (used by the category pages).
- **Offset paging, 50 per page (max 100), offset capped at 10,000, with `total` and `has_more`.** Simple at personal scale; can skip or repeat an item if one is added between pages. Alternative: keyset paging.
- **Sorts:** newest, oldest (by `created_at`, the "added" date), name A–Z / Z–A (case-insensitive), category (category name, then item name). Unknown sort → 400. "Updated" isn't offered; easy to add.
- **Category filter:** comma-separated and/or repeated `category=` slugs, trimmed and de-duplicated, max 20; a malformed slug → 400; an unknown or not-yours slug just matches nothing (categories are public, so no existence leak).
- **Pin/unpin are idempotent and return 204.** Pinning an unknown category is 400 "unknown category", matching `POST /me/collections`. `POST` + `DELETE` instead of `PUT` because the CORS config doesn't allow `PUT`.

## Web

- **`/my` is the overview** (not a separate `/my/overview`), with a Collections / All items switch. Why: one place to review what you own; the existing collection list becomes the grouped "Collections" view, and inline search keeps working (search replaces the overview while it has text, as before).
- **URL holds `view`, `sort`, `category`; defaults are omitted** so plain `/my` is unchanged. Discrete changes `router.push` (Back undoes them); search text still `router.replace`s. Unknown URL values are ignored, not errors.
- **Filter chips are multi-select** with an "All" chip that clears; they apply to both views. They scroll horizontally on phones.
- **"Start a collection" on an empty added category creates a collection named after the category and opens it**, one tap instead of a form; the user can rename it there. Alternative: prefill the New collection form.
- **The New collection form moved below the category groups**, so a returning collector sees their collections first.
- **Add a category is an inline panel, not a modal**, listing every category with Add / Added + Remove / "In my collection" (a link to the filtered overview, since unpinning a category you collect wouldn't hide it). The Categories list and each category page have the same button; the category page's stale "Sign-in coming soon" note is replaced by it (or a sign-in prompt).
- **The show-empty switch updates optimistically** and reverts with an error if saving fails.
- **Item dates use the viewer's locale and time zone** (client-only data, so no hydration mismatch).

## Seed

- **Dev items got fixed, spread-out `created_at` dates** so date sorts have a visible order (they were all inserted at the same instant). The items insert now upserts only `created_at` on conflict, guarded by `IS DISTINCT FROM`, so re-runs stay idempotent and existing local databases pick the dates up.

## Not done

- No per-category hide, no "recently updated" sort, no search box inside the Add a category panel (8 categories today; worth adding when user-built categories grow the list).
- Nothing in scope is unfinished.
