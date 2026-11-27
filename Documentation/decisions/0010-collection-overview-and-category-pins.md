# ADR 0010: Collection overview and category pins

- **Status:** Proposed
- **Date:** 2026-11-27

## Context

The owner asked for a "collection overview": review everything you've added, sortable by time and name and filterable by category, with categories you have nothing in shown or hidden on request, and a way to add any approved global category to your view before you own anything in it. Collections are per-category containers that hold items; a user can have several in one category. Two later features constrain the model: the two-layer catalog (ADR 0004) and user-built categories (usable by their creator at once, global only after admin approval).

## Decision

- **"Adding" a category is a pin, not an empty collection.** New table `user_pinned_categories(user_id, category_id, created_at)`, PK on both ids, both FKs `ON DELETE CASCADE`. A row exists only while the category is pinned. A user's overview categories are the union of categories they have collections in and categories they pinned.
  - Why not create an empty collection: a collection is a user-named container that appears on the public profile (ADR 0009) and would need a made-up name. Pins are a private view preference, so they never leak to profile viewers, never need a name, and never touch items or the catalog layer.
  - "Start a collection" from an empty pinned category creates a collection named after the category; the user renames it on the collection page.
- **"Show empty categories" is one per-user boolean**, `users.show_empty_categories` (default `true`, so a category you just added is visible). "Empty" means zero items, so a category whose collections are all empty is hidden too; the overview says how many are hidden and offers "Show them". No per-category hide flag: the request was a single show/hide toggle, and a column can be added later if per-category hiding is wanted.
- **One read endpoint**, `GET /me/overview?sort=&category=&limit=&offset=`, returns the overview categories (counts, pinned flag), the setting, and one page of items (default 50, max 100; `limit=0` for categories only; offset capped at 10,000). Sorts are a fixed allowlist mapped to fixed `ORDER BY` clauses, each ending in the item id so paging is stable. Writes: `POST /me/pinned-categories {category_slug}`, `DELETE /me/pinned-categories/{slug}` (both idempotent, 204), `PATCH /me/overview-settings {show_empty_categories}`.
- **`/my` is the overview**, with a Collections / All items switch. The URL (`view`, `sort`, `category`) is the source of truth; each change pushes a history entry so Back undoes it.

## Consequences

- When user-built categories land, `Overview.Pin` must accept only categories the user may use (approved, or their own), and the overview must not show another user's unapproved category. Pins referencing a deleted category disappear by cascade.
- Offset paging can skip or repeat an item if items are added between pages; acceptable for personal collections, revisit with keyset paging if needed.
