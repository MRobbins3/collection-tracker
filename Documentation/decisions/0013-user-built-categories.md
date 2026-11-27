# ADR 0013: User-built categories

- **Status:** Proposed
- **Date:** 2026-11-27

## Context

The owner asked that users can build their own categories by selecting existing fields (e.g. from Lego) and adding new values for fields when they don't find what they want. Owner decision: a user-built category is usable at once by its creator (and visible to people allowed to see the creator's collections, per `api/internal/privacy`), and joins the global category list only after admin approval. Categories carry a JSON Schema `attribute_schema` that the item form (`AttributeFields`) renders and the API validates (`category.ValidateAttributes`). ADR 0010 asked that pins be restricted once categories have owners; ADR 0012 built the moderation queue to take new kinds.

## Decision

- **Categories gain ownership and a lifecycle** (migration 0008): `created_by` (NULL for system categories, `ON DELETE SET NULL`), `status` in `private | pending | approved | rejected`, `approved_by` / `approved_at`, timestamps, and a generated `name_key` (the same Unicode-aware expression as `catalog_entries.name_key`, matching `textnorm.Key`). Existing rows are approved system categories. A CHECK refuses an unapproved category without an owner. Unique indexes: one approved category per `name_key`, and one per `(created_by, name_key)`.
- **One visibility rule, in one place:** a category is visible to everyone when approved and to its creator always. It is the `visibleTo` SQL fragment in `api/internal/store/category.go`, used by every read on behalf of a user: category list, search, and page; catalog browse; new collections; pins and the overview's category list. Another user's unapproved category is 404, the same as an unknown slug. Catalog suggestions require an approved category, so unapproved categories never have catalog entries.
- **Collection viewers see the category through the collection, not by slug.** `GET /users/{handle}/collections/{id}` embeds the collection's category (name and schema) once the privacy rule has allowed the viewer to see the collection. `GET /categories/{slug}` stays approved-or-yours, so a private category is never discoverable on its own.
- **Fields come from a library, not free-form schema editing.** `GET /category-fields` derives reusable fields from approved categories' schemas: properties de-duplicated by normalized title + type (`<type>:<snake_case title>` id), listing every category that uses each and that category's options. A builder request names field ids plus option lists; the server composes the schema (`category.Compose`) from library definitions only, so titles, types, and constraints can't be forged. Options are allowed on text fields and become a JSON Schema `enum` (rendered as a select): trimmed and collapsed, de-duplicated case-insensitively, reusing an existing library spelling; casing is otherwise the user's (the UI offers Title Case). Limits: 20 fields, 50 options, 60 characters per option. Brand-new custom fields are not supported yet.
- **Names are normalized and reused, not duplicated:** a new name matching (case- and space-insensitively) a category the user can already see is refused with that category (409 with `existing`), and the UI links to it. Slugs are the folded name plus a random 6-hex suffix, fixed for life, so private categories never collide and names can't be probed by slug.
- **Lifecycle:** create (private, usable at once), edit while private or rejected (an edit can't remove a field or an option that the creator's items still use), submit (`POST /me/categories/{slug}/submit` → pending, a `moderation_requests` row of kind `category`, counting toward the 20-request cap), admin approve (approved, on the public list) or reject with a reason (rejected: private again, editable, resubmittable; the reason shows on the category page and in My submissions). Pending categories can't be edited, so the admin reviews what gets published. Categories are approved as built; admins ask for changes by rejecting.
- **After approval the category is fixed for its creator.** Changes to an approved category (e.g. new options) will go through review, like catalog corrections; not built yet.
- **Account deletion** removes the user's unapproved categories (and their collections in them); approved categories stay, unattributed.

## Consequences

- Every new query that reads categories for a user must use `visibleTo` (documented in CLAUDE.md).
- The server render has no cookie, so it sees approved categories only; web fetches of category lists and pages watch the signed-in state and load again once auth resolves, keeping hydration parity.
- The field library grows as user categories are approved; their options join each field's merged option list, but a picked field starts with only its source category's options.
- jsonb doesn't keep key order, so a user category's fields show in jsonb order, like the seeded categories.
- Rolling back 0008 deletes unapproved user categories with their collections rather than making them public.
