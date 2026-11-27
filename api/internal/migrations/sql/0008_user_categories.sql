-- +goose Up
-- +goose StatementBegin

-- User-built categories (ADR 0013).
--
-- 1. Ownership and lifecycle. A category is either a system category
--    (created_by NULL, from the reference seed) or one a user built.
--    status:
--      private   the creator built it and uses it; nobody else sees it
--      pending   the creator asked for it to join the public list
--      approved  on the public list for everyone (every system category)
--      rejected  an admin declined it; the creator still uses it privately
--    Existing rows are system categories and become approved. The default
--    stays 'approved' so the reference seed's inserts need no change; the
--    one store path that creates a user category sets 'private' explicitly,
--    and the CHECK below refuses an unapproved category with no owner.
--    created_by is SET NULL on account deletion so an approved category
--    stays (unattributed); Users.Delete removes the unapproved ones first.
ALTER TABLE categories
    ADD COLUMN created_by  uuid REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN status      text NOT NULL DEFAULT 'approved'
                             CHECK (status IN ('private', 'pending', 'approved', 'rejected')),
    ADD COLUMN approved_by uuid REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN approved_at timestamptz,
    ADD COLUMN created_at  timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN updated_at  timestamptz NOT NULL DEFAULT now(),
    ADD CONSTRAINT categories_unapproved_have_owner
        CHECK (status = 'approved' OR created_by IS NOT NULL);

-- 2. name_key: the case- and space-insensitive comparison form of the name,
--    the same expression as catalog_entries.name_key (0006), which matches
--    textnorm.Key in the API.
ALTER TABLE categories
    ADD COLUMN name_key text NOT NULL GENERATED ALWAYS AS (
        lower(regexp_replace(
            regexp_replace(name,
                '^[\s\u0085   -     　]+|[\s\u0085   -     　]+$',
                '', 'g'),
            '[\s\u0085   -     　]+', ' ', 'g'))
    ) STORED,
    ADD CONSTRAINT categories_name_not_blank CHECK (name_key <> '');

-- One public category per name, and one per name among a user's own. Two
-- users may each have a private "Sneakers"; only one can be approved (the
-- second approval fails and the admin rejects it as a duplicate).
CREATE UNIQUE INDEX categories_approved_name_key
    ON categories (name_key) WHERE status = 'approved';
CREATE UNIQUE INDEX categories_owner_name_key
    ON categories (created_by, name_key) WHERE created_by IS NOT NULL;
CREATE INDEX categories_created_by_idx ON categories (created_by);

-- 3. A new moderation kind: a user asks for their category to join the
--    public list. category_id is the category itself; there is no catalog
--    entry. One pending request per category at a time.
ALTER TABLE moderation_requests
    DROP CONSTRAINT moderation_requests_kind_check,
    ADD CONSTRAINT moderation_requests_kind_check
        CHECK (kind IN ('catalog_entry', 'catalog_correction', 'category')),
    ADD CONSTRAINT moderation_requests_category_has_no_entry
        CHECK (kind <> 'category' OR catalog_entry_id IS NULL);
CREATE UNIQUE INDEX moderation_requests_one_pending_per_category
    ON moderation_requests (category_id) WHERE kind = 'category' AND status = 'pending';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Without ownership every category would be public, so unapproved user
-- categories are deleted (with their collections, items, and pins) rather
-- than leaked. Approved user categories stay as ordinary categories.
DELETE FROM moderation_requests WHERE kind = 'category';
DROP INDEX IF EXISTS moderation_requests_one_pending_per_category;
ALTER TABLE moderation_requests
    DROP CONSTRAINT IF EXISTS moderation_requests_category_has_no_entry,
    DROP CONSTRAINT moderation_requests_kind_check,
    ADD CONSTRAINT moderation_requests_kind_check
        CHECK (kind IN ('catalog_entry', 'catalog_correction'));
DELETE FROM collections c
USING categories cat
WHERE c.category_id = cat.id AND cat.status <> 'approved';
DELETE FROM categories WHERE status <> 'approved';
DROP INDEX IF EXISTS categories_created_by_idx;
DROP INDEX IF EXISTS categories_owner_name_key;
DROP INDEX IF EXISTS categories_approved_name_key;
ALTER TABLE categories
    DROP CONSTRAINT IF EXISTS categories_name_not_blank,
    DROP CONSTRAINT IF EXISTS categories_unapproved_have_owner,
    DROP COLUMN IF EXISTS name_key,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS created_at,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS created_by;
-- +goose StatementEnd
