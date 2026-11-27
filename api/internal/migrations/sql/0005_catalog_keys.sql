-- +goose Up
-- +goose StatementBegin

-- Natural keys for the global catalog (ADR 0004, phase 2 seeding).
--
-- categories.catalog_identifier names the attribute (a key in the
-- category's attribute_schema) that, together with the name, tells two
-- catalog entries apart: a Lego set's set number, a book's ISBN, a record's
-- artist. NULL means the name alone identifies an entry. The reference seed
-- sets it per category.
ALTER TABLE categories
    ADD COLUMN catalog_identifier text;

-- catalog_entries.identifier is that attribute's value, copied from
-- attributes when the entry is written (the seed does it in SQL, the API in
-- Go). It is display text ("75192"); the *_key columns are the normalized
-- comparison forms: trimmed, internal whitespace collapsed, lowercased.
-- 0006 widens the whitespace handling to Unicode spaces so they line up
-- with textnorm.Key in the API.
ALTER TABLE catalog_entries
    ADD COLUMN identifier text,
    ADD COLUMN name_key text NOT NULL
        GENERATED ALWAYS AS (lower(regexp_replace(btrim(name), '\s+', ' ', 'g'))) STORED,
    ADD COLUMN identifier_key text NOT NULL
        GENERATED ALWAYS AS (lower(regexp_replace(btrim(coalesce(identifier, '')), '\s+', ' ', 'g'))) STORED,
    ADD CONSTRAINT catalog_entries_name_not_blank CHECK (btrim(name) <> '');

-- One entry per (category, name, identifier). An entry without an
-- identifier keys on '' so it is unique by name alone. The seed upserts on
-- this; the moderation flow will reuse it to spot duplicate submissions.
ALTER TABLE catalog_entries
    ADD CONSTRAINT catalog_entries_natural_key UNIQUE (category_id, name_key, identifier_key);

-- Browse lists approved entries in a category, ordered by name.
CREATE INDEX catalog_entries_browse_idx
    ON catalog_entries (category_id, status, name_key, identifier_key, id);
CREATE INDEX catalog_entries_identifier_trgm
    ON catalog_entries USING GIN (identifier gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS catalog_entries_identifier_trgm;
DROP INDEX IF EXISTS catalog_entries_browse_idx;
ALTER TABLE catalog_entries
    DROP CONSTRAINT IF EXISTS catalog_entries_natural_key,
    DROP CONSTRAINT IF EXISTS catalog_entries_name_not_blank,
    DROP COLUMN IF EXISTS identifier_key,
    DROP COLUMN IF EXISTS name_key,
    DROP COLUMN IF EXISTS identifier;
ALTER TABLE categories DROP COLUMN IF EXISTS catalog_identifier;
-- +goose StatementEnd
