-- +goose Up
-- +goose StatementBegin

-- People search (ADR 0014) matches lower(display_name) anywhere and handle
-- by prefix with LIKE; trigram indexes keep both off a full users scan.
-- pg_trgm is installed by 0001.
CREATE INDEX users_display_name_lower_trgm ON users USING GIN (lower(display_name) gin_trgm_ops);
CREATE INDEX users_handle_trgm ON users USING GIN (handle gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_handle_trgm;
DROP INDEX IF EXISTS users_display_name_lower_trgm;
-- +goose StatementEnd
