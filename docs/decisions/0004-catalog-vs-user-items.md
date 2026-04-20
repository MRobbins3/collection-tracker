# ADR 0004: Two-layer model — catalog entries and user items

- **Status:** Accepted (directional; partial implementation lands with Milestone 8, full catalog population is phase 2)
- **Date:** 2026-04-19

## Context

The product vision is: a user opens the app, picks a category, and finds an already-known list of *things that exist in the world* (every Lego set, every Funko Pop, every US coin denomination, every houseplant species). They don't retype what Wikipedia or Brickset already knows — they just check the box for what they own. If the thing they want isn't in the list yet, they can enter it freeform now and optionally submit it as a supplement to the global catalog so the next user benefits.

That shape implies two distinct data concepts:

1. **Catalog entry** — a canonical "this thing exists" record. One per real-world item per category. Shared across all users. Carries rich attributes (Lego set 75192 → 7541 pieces, UCS Millennium Falcon, theme Star Wars, year 2017). Can be seeded by us, submitted by users, or imported from external catalogs (Brickset, Discogs, etc.).

2. **User item** — a specific user's ownership record. References a catalog entry *when available* and adds per-owner fields: quantity, condition, personal notes, acquired date. A single catalog entry can be owned by many users; each user's ownership record is independent.

Today's `items` table collapses both layers. Its `name` + `attributes` are free-form per user. That works for MVP but conflicts with the vision: when the catalog lands, we'll want every existing item to *either* point at a catalog entry *or* stay free-form (while the user finds a match or submits a new one).

## Decision

Commit to the two-layer model conceptually now. Build only what Milestone 8 needs, but in a forward-compatible shape.

**Milestone 8 (imminent):**
- Keep `items` as the user-owned layer. Items carry `name`, `quantity`, `condition`, `attributes`.
- Add `items.catalog_entry_id uuid NULL REFERENCES catalog_entries(id) ON DELETE SET NULL`. Nullable from day one. Every item created in MVP has `catalog_entry_id = NULL`.
- Introduce `catalog_entries` as an (initially empty) table. Rows carry `category_id`, `name`, `attributes` (same JSONB shape the category's `attribute_schema` describes), plus:
  - `source text NOT NULL DEFAULT 'user_submitted'` — one of `seed | user_submitted | import`
  - `status text NOT NULL DEFAULT 'pending'` — one of `pending | approved | rejected`
  - `submitted_by uuid NULL REFERENCES users(id) ON DELETE SET NULL` — who created the submission
  - `approved_by uuid NULL REFERENCES users(id) ON DELETE SET NULL` — which admin approved it
  - `approved_at timestamptz NULL` — when it was approved
  The last two are populated when roles/moderation land in phase 2. Carrying them from day one means the moderation flow drops in without a schema change.
- The item-add UI is shaped like a search combobox with "can't find it — enter manually" as the escape hatch. Today the search returns nothing (catalog is empty), so every add falls through to free-form. Later we populate the catalog and the autocomplete starts doing work.

**Phase 2 (post-MVP, separate milestones):**
- Seed the catalog per category (hand-curated, imported from partners, or both).
- Users can submit new catalog entries from the "can't find it" escape hatch. New entries land with `status = pending` and `source = user_submitted` for moderator review.
- Backfill existing user items by matching names/attributes to catalog entries (loose join + user-confirmation flow).
- Add a separate `catalog_supplements` or similar flow for users proposing *corrections* to existing entries, with an audit trail.

**Roles and moderation (phase 2, but the shape is decided now):**
- The vast majority of users have no role — "regular user" is implicit (null/absent role).
- A small subset are **admins** who can review submissions, approve or reject catalog entries, and edit existing entries.
- Implementation intent: add a `users.role text NULL` column (values `admin`, extensible later) in the migration that turns on moderation. Nullable so the migration is cheap and most rows stay untouched.
- Auth middleware gains a `RequireRole("admin")` check that sits on top of `Require`.
- The `approved_by` / `approved_at` columns on `catalog_entries` (added in Milestone 8) become the audit trail — no migration needed at phase-2 time.
- Admin-only routes (moderation queue, approve/reject) live behind the new middleware. They do not exist in MVP.

## Consequences

**Positive**
- Zero data migration needed when the catalog populates — items already have the FK slot.
- `source` + `status` on catalog entries means the moderation story drops in without a schema change.
- Category `attribute_schema` stays the single contract — same shape validates catalog entries, seeded attributes, and user item attributes.
- UI shape today anticipates tomorrow: the autocomplete is empty in MVP, not missing, so adding search is a data change, not a UI rebuild.

**Negative / trade-offs**
- Carrying a nullable FK and two extra columns (`source`, `status`) on a table with no catalog content yet is a little bit of future-proofing overhead. Acceptable given how painful it'd be to add later.
- Users with MVP-era items will have `catalog_entry_id = NULL` even when a catalog entry for "their thing" eventually exists. We'll need a backfill / link-me-up flow.
- Keeping `items.name` and `items.attributes` mutable per user even when a catalog match exists is deliberate (users personalize: "missing the dish", "signed copy"), but it means the rendered item is always "the user's record, possibly informed by the catalog" rather than "the catalog entry itself."

## Revision triggers

Open a new ADR and flip this one if:
- We decide to merge catalog + items into a single denormalized "ownership" concept instead of the two-layer model.
- We adopt an external catalog provider (Brickset, Discogs) as the primary source, in which case the schema for `catalog_entries` needs an `external_id` + `external_source` namespacing.
- The moderation story for user-submitted entries needs more than a simple `status` enum (e.g., per-user reputation, per-edit review).
