# Category Attribute Schemas

The per-category JSON Schemas are the source of truth for:

1. The dynamic form rendered on the frontend when adding/editing an item (`AttributeFields.vue`).
2. Server-side validation of `items.attributes` (JSONB) at the API edge via `internal/category.ValidateAttributes`.
3. The `categories.attribute_schema` JSONB column, seeded + upserted idempotently on every API startup.

## Where the schemas actually live

All eight category schemas ship in a single file:

- **`api/internal/seed/data/categories.json`** — embedded into the Go binary with `go:embed`.

The original plan imagined one JSON file per category under `docs/categories/` with a mirror seeded into the DB. That was over-engineered for eight categories. Collapsing to one embedded seed file keeps the surface small and means "add a category" is a one-file edit.

When the catalog-seeding work (phase 2) starts, per-category *data* files (actual catalog entries — Lego sets, Funko Pops, etc.) will live elsewhere, likely under `api/internal/seed/catalog/`. The schemas stay where they are.

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
- `title` — **required** (enforced by review, not by the seed loader). The label the user sees above the input, and the `dt` on the read-only detail view.
- `description` — optional short hint shown under the label and under the read-only dt. Keep under ~80 chars.
- Constraints (`minimum`, `maximum`, `pattern`, `enum`, etc.) are JSON Schema and enforced server-side by `gojsonschema`; the frontend doesn't re-implement them (yet).

The UI falls back to a humanized version of the property name if `title` is missing (e.g. `set_number` → `Set number`) so an absence of `title` degrades gracefully rather than leaking jargon.

## Adding a new category

1. Append an entry to `api/internal/seed/data/categories.json` with slug, name, description, and `attribute_schema` (include `title` + optional `description` on every property).
2. Restart the API. The seed runs on startup and idempotently UPSERTs; existing rows keyed by slug get their name/description/attribute_schema refreshed.
3. If the category's attribute set needs a new UI input type (e.g., date, color, enum picker), extend `AttributeFields.vue` to handle it.
