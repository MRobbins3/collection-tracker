-- +goose Up
-- +goose StatementBegin

-- The global catalog of "known things" per category. Empty in MVP; phase 2
-- populates it via seeds, user submissions, and imports. Items can reference
-- a catalog entry but don't have to (free-form items stay valid forever).
CREATE TABLE catalog_entries (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id  uuid NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    name         text NOT NULL,
    description  text,
    attributes   jsonb NOT NULL DEFAULT '{}'::jsonb,
    source       text NOT NULL DEFAULT 'user_submitted'
                   CHECK (source IN ('seed', 'user_submitted', 'import')),
    status       text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'approved', 'rejected')),
    submitted_by uuid REFERENCES users(id) ON DELETE SET NULL,
    approved_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    approved_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX catalog_entries_category_idx ON catalog_entries(category_id);
CREATE INDEX catalog_entries_status_idx   ON catalog_entries(status);
CREATE INDEX catalog_entries_name_trgm    ON catalog_entries USING GIN (name gin_trgm_ops);
CREATE INDEX catalog_entries_attrs_gin    ON catalog_entries USING GIN (attributes jsonb_path_ops);

-- Items can reference a catalog entry. Nullable — MVP has none and free-form
-- items always stay valid. ON DELETE SET NULL so removing a catalog entry
-- doesn't delete user items; it just converts them to free-form.
ALTER TABLE items
    ADD COLUMN catalog_entry_id uuid REFERENCES catalog_entries(id) ON DELETE SET NULL;
CREATE INDEX items_catalog_idx ON items(catalog_entry_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE items DROP COLUMN IF EXISTS catalog_entry_id;
DROP TABLE IF EXISTS catalog_entries;
-- +goose StatementEnd
