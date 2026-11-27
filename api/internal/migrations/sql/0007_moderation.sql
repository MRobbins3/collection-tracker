-- +goose Up
-- +goose StatementBegin

-- Roles and moderation (ADR 0004 "Roles and moderation", ADR 0012).
--
-- 1. users.role: NULL for almost everyone ("regular user" is implicit).
--    'admin' reviews catalog submissions and corrections. There is no
--    self-service way to get a role; see ADR 0012 for the SQL an operator
--    runs to appoint an admin.
ALTER TABLE users
    ADD COLUMN role text CHECK (role IN ('admin'));

-- 2. The catalog natural key (0005) applies to live entries only: pending
--    and approved. A rejected submission (or one merged into an existing
--    entry) stays as history for its submitter but no longer reserves its
--    name, so a later, better submission isn't blocked. Pending entries keep
--    the key, so two users can't queue the same thing twice.
ALTER TABLE catalog_entries
    DROP CONSTRAINT catalog_entries_natural_key;
CREATE UNIQUE INDEX catalog_entries_natural_key
    ON catalog_entries (category_id, name_key, identifier_key)
    WHERE status IN ('pending', 'approved');

-- 3. The moderation queue: one row per thing a user asked admins to review.
--    `kind` says what it is; each kind uses the columns it needs, so later
--    kinds (user-built categories) add a value to the CHECK and, if needed,
--    a nullable reference column, without a second queue.
--
--    catalog_entry       a new entry the user submitted. catalog_entry_id is
--                        the submitted (pending) entry row itself.
--    catalog_correction  changes to an approved entry. catalog_entry_id is
--                        the target; `proposed` holds only the changed
--                        fields ({"name", "description", "attributes"}).
--
--    The decision columns are the audit trail: who decided, when, the
--    outcome (status), the reason shown to the submitter, and for changes
--    the entry's values before (`previous`) and what was applied
--    (`applied`, which differs from `proposed` after edit-then-approve).
CREATE TABLE moderation_requests (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind             text NOT NULL CHECK (kind IN ('catalog_entry', 'catalog_correction')),
    status           text NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'approved', 'rejected', 'merged')),
    category_id      uuid NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    catalog_entry_id uuid REFERENCES catalog_entries(id) ON DELETE CASCADE,
    -- A user's requests are their data: deleting the account deletes them
    -- (ADR 0009: tables referencing users cascade).
    submitted_by     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- The free-form item a submission came from. On approval (or merge) it
    -- is linked to the entry if it is still free-form; ADR 0012.
    source_item_id   uuid REFERENCES items(id) ON DELETE SET NULL,
    proposed         jsonb,
    note             text,
    decided_by       uuid REFERENCES users(id) ON DELETE SET NULL,
    decided_at       timestamptz,
    decision_reason  text,
    merged_into_id   uuid REFERENCES catalog_entries(id) ON DELETE SET NULL,
    previous         jsonb,
    applied          jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT moderation_requests_entry_kinds_have_entry
        CHECK (kind NOT IN ('catalog_entry', 'catalog_correction') OR catalog_entry_id IS NOT NULL),
    CONSTRAINT moderation_requests_correction_has_proposal
        CHECK (kind <> 'catalog_correction' OR proposed IS NOT NULL),
    CONSTRAINT moderation_requests_decided
        CHECK ((status = 'pending') = (decided_at IS NULL)),
    CONSTRAINT moderation_requests_merge_is_for_entries
        CHECK (status <> 'merged' OR (kind = 'catalog_entry' AND decided_at IS NOT NULL))
);

-- One request per submitted entry.
CREATE UNIQUE INDEX moderation_requests_one_per_entry
    ON moderation_requests (catalog_entry_id) WHERE kind = 'catalog_entry';
-- The admin queue: pending, oldest first, id as the tie-breaker.
CREATE INDEX moderation_requests_queue_idx
    ON moderation_requests (created_at, id) WHERE status = 'pending';
-- "My submissions", and the per-user pending cap.
CREATE INDEX moderation_requests_submitter_idx
    ON moderation_requests (submitted_by, created_at DESC, id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS moderation_requests;
-- Fails if rejected rows now share a key with live ones; delete those
-- first (they are history only).
DROP INDEX IF EXISTS catalog_entries_natural_key;
ALTER TABLE catalog_entries
    ADD CONSTRAINT catalog_entries_natural_key UNIQUE (category_id, name_key, identifier_key);
ALTER TABLE users DROP COLUMN IF EXISTS role;
-- +goose StatementEnd
