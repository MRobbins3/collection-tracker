# Category Attribute Schemas

Each category has a JSON Schema describing its category-specific item attributes. These schemas are the source of truth for:

1. The dynamic form rendered on the frontend when adding/editing an item.
2. Server-side validation of `items.attributes` (JSONB) at the API edge.
3. The `categories.attribute_schema` column, seeded from these files.

Filenames follow the category `slug`: e.g., `lego-sets.json`, `funko-pops.json`.

Schemas arrive alongside the `feat(api): seed categories + public browse endpoints` milestone.
