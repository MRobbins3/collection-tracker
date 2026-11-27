# Category Attribute Schemas

The per-category JSON Schemas are the source of truth for:

1. The dynamic form rendered on the frontend when adding/editing an item (`AttributeFields.vue`).
2. Server-side validation of `items.attributes` (JSONB) at the API edge via `internal/category.ValidateAttributes`.
3. The `categories.attribute_schema` JSONB column, seeded and upserted idempotently on every API startup.

## Where the schemas actually live

All eight system category schemas ship in a single reference seed file:

- **`api/internal/seed/sql/reference/0001_categories.sql`**, embedded into the Go binary with `go:embed` and applied after the migrations on every start.

It is plain SQL (one `INSERT ... ON CONFLICT (slug) DO UPDATE`), so it can also be run by hand with `psql`. `Documentation/seed-data.md` covers the seed sets, the rules every seed file follows, and how to rebuild the database from scratch; ADR 0007 records why seeds are SQL.

The same file sets each category's `catalog_identifier`: the attribute that, with the name, tells two catalog entries apart (`set_number` for Lego Sets, `artist` for Vinyl Records). Catalog entries copy that attribute's value into `catalog_entries.identifier` and are unique per category on normalized name + identifier (ADR 0011); compare uses it to pick between entries that share a name. A category without one keys its entries on name alone.

Catalog entries are not reference data yet. A small cited sample (Lego sets, vinyl records, US coins) lives in the dev seed set, **`api/internal/seed/sql/dev/0002_sample_catalog.sql`**, so it never reaches production; the real dataset is pending and will replace it. The schemas stay in `0001_categories.sql`.

## User-built categories

Users can also build their own categories in the app (ADR 0013). Those rows are created by the API, not the seed: their schema is assembled from fields approved categories already use (the creator may pick one as the `catalog_identifier`, "the detail that tells items apart"), they start `private` to the creator, and they join the public list only when an admin approves them. The reference seed never touches them, and skips a seeded name an approved user category already holds.

## Property shape (non-engineer-friendly copy)

Every property in `attribute_schema.properties` must carry a human label. Bare `snake_case` property names must never reach the UI — the app is for non-engineers.

```json
{
  "set_number": {
    "type": "string",
    "title": "Set number",
    "description": "The number printed on the box, e.g. 75192."
  }
}
```

- `type` — one of `string`, `integer`, `number`, `boolean`. Renders the right input in `AttributeFields.vue`.
- `title` — **required** (enforced by an integration test in `api/internal/seed/categories_integration_test.go` and by review). The label the user sees above the input, and the `dt` on the read-only detail view.
- `description` — optional short hint shown under the label and under the read-only dt. Keep under ~80 chars.
- Constraints (`minimum`, `maximum`, `pattern`, `enum`, etc.) are JSON Schema and enforced server-side by `gojsonschema`; the frontend doesn't re-implement them (yet).

The UI falls back to a humanized version of the property name if `title` is missing (e.g. `set_number` → `Set number`) so an absence of `title` degrades gracefully rather than leaking jargon.

## Adding a new category

1. Add a row to the `VALUES` list in `api/internal/seed/sql/reference/0001_categories.sql` with slug, name, description, and `attribute_schema` (include `title` + optional `description` on every property).
2. Add the category to the expectations in `TestReferenceSeedsProduceCuratedCategories` (`api/internal/seed/categories_integration_test.go`).
3. Restart the API (`docker compose restart api`). The seed runs on startup and idempotently upserts; existing rows keyed by slug get their name/description/attribute_schema refreshed.
4. If the category's attribute set needs a new UI input type (e.g., date, color, enum picker), extend `AttributeFields.vue` to handle it.

Removing a category from the file does not delete it from the database; write a goose migration for that.
