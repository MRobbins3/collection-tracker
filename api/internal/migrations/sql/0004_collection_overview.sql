-- +goose Up
-- +goose StatementBegin

-- Collection overview preferences.
--
-- user_pinned_categories: categories a user added to their overview before
-- (or without) starting a collection in them. A row exists only while the
-- category is pinned, so there is no boolean column. It is a view preference,
-- not a collection: it holds no items, never appears on the public profile,
-- and doesn't touch the two-layer catalog (ADR 0004). Both FKs cascade, so
-- DELETE /me stays complete and removing a (future user-built) category
-- drops its pins.
CREATE TABLE user_pinned_categories (
    user_id     uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id uuid        NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, category_id)
);

-- show_empty_categories: whether the overview lists categories with no items
-- (pinned ones and ones whose collections are all empty). Default on, so a
-- category you just added is visible.
ALTER TABLE users
    ADD COLUMN show_empty_categories boolean NOT NULL DEFAULT true;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN IF EXISTS show_empty_categories;
DROP TABLE IF EXISTS user_pinned_categories;
-- +goose StatementEnd
