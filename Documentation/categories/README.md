# Category Attribute Schemas

The per-category JSON Schemas are the source of truth for:

1. The dynamic form rendered on the frontend when adding/editing an item (`AttributeFields.vue`).
2. Server-side validation of `items.attributes` (JSONB) at the API edge via `internal/category.ValidateAttributes`.
3. The `categories.attribute_schema` JSONB column, seeded and upserted idempotently on every API startup.

## Where the schemas actually live

All eight category schemas ship in a single reference seed file:

- **`api/internal/seed/sql/reference/0001_categories.sql`**, embedded into the Go binary with `go:embed` and applied after the migrations on every start.

It is plain SQL (one `INSERT ... ON CONFLICT (slug) DO UPDATE`), so it can also be run by hand with `psql`. `Documentation/seed-data.md` covers the seed sets, the rules every seed file follows, and how to rebuild the database from scratch; ADR 0007 records why seeds are SQL.

Catalog entries (actual Lego sets, Funko Pops, etc.) will be further numbered files in the same `reference/` directory when the catalog-seeding work starts. The schemas stay in `0001_categories.sql`.

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
