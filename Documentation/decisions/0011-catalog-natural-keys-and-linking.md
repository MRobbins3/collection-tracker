# ADR 0011: Catalog natural keys and explicit item linking

- **Status:** Deferred
- **Date:** 2026-11-27

## Context

ADR 0004 designed the two-layer catalog (approved `catalog_entries` + user `items` with a nullable `catalog_entry_id`) but left the catalog empty and gave entries no natural key. Phase 2 needs one: the seed must be idempotent (ADR 0007), the moderation flow will need to spot duplicate submissions, and the brief's normalization rule says user-typed values that others match on must not differ only by case or spacing. Names alone are not unique within a category: Lego has several "Millennium Falcon" sets, and many artists have a "Greatest Hits".

## Decision

- **Per-category identifier attribute.** `categories.catalog_identifier` (nullable) names the attribute in the category's `attribute_schema` that, together with the name, tells entries apart: Lego `set_number`, Funko `number`, stamps `catalog_id`, trading cards `set_name`, books `isbn`, vinyl `artist`. Coins and plants have none, so the name alone is the key. The reference seed owns the values; a seed test checks each names a real schema property.
- **Natural key on entries.** `catalog_entries.identifier` holds the identifier attribute's value as display text (copied from `attributes` on write: by the seed in SQL, later by the API in Go). Generated columns `name_key` and `identifier_key` hold the normalized forms (trim, collapse whitespace, lowercase, matching `textnorm.Key`), and `UNIQUE (category_id, name_key, identifier_key)` is the key. An entry without an identifier keys on `''`.
- **Only approved entries are public.** Every store read filters `status = 'approved'`; pending or rejected entries are 404, like unknown ones. Items can only link to approved entries in their collection's category.
- **Linking is always explicit.** Items link at creation (picked from the catalog) or through `PUT …/items/{id}/catalog-entry`; `DELETE` unlinks. Editing an item (`PATCH`) never changes the link. Suggestions (`GET …/catalog-suggestions`) rank same name + identifier, same identifier, same name, then pg_trgm-similar names (≥ 0.3), and never link anything themselves. Linking never rewrites the owner's name or attributes (ADR 0004: users personalize).
- **The sample catalog is dev seed data**, not reference data, until the owner's real dataset lands; production starts with an empty catalog.

## Consequences

- The seed and future submissions share one duplicate rule enforced by the database, not by convention.
- `identifier` is denormalized from `attributes`; anything writing entries must copy it (the seed does it in SQL; the moderation PR must do it in Go and should reuse `textnorm`).
- Changing a category's `catalog_identifier` later changes what its entries' `identifier` should be; that needs a migration to recompute them, not just a seed edit.
- The identifier is a disambiguator, not a unique id on its own: two entries may share a set number if their names differ. If an external catalog becomes the source of truth, add `external_source` + `external_id` (ADR 0004 revision trigger).
