-- +goose Up
-- +goose StatementBegin

-- Review fixes for 0005 (kept as a separate migration so 0005 stays as it
-- was applied on existing dev databases).
--
-- 1. Normalize Unicode whitespace like the API's textnorm.Clean (Go's
--    unicode.IsSpace set: ASCII whitespace, U+0085, NBSP U+00A0, U+1680,
--    U+2000–U+200A, U+2028, U+2029, U+202F, U+205F, U+3000), not just the
--    ASCII spaces btrim and \s cover, so "Abbey Road" and "Abbey Road"
--    share a key. Lowercasing is Postgres lower() under the database
--    collation, which agrees with Go's strings.ToLower for ordinary text.
ALTER TABLE catalog_entries
    ALTER COLUMN name_key SET EXPRESSION AS (
        lower(regexp_replace(
            regexp_replace(name,
                '^[\s\u0085   -     　]+|[\s\u0085   -     　]+$',
                '', 'g'),
            '[\s\u0085   -     　]+', ' ', 'g'))
    ),
    ALTER COLUMN identifier_key SET EXPRESSION AS (
        lower(regexp_replace(
            regexp_replace(coalesce(identifier, ''),
                '^[\s\u0085   -     　]+|[\s\u0085   -     　]+$',
                '', 'g'),
            '[\s\u0085   -     　]+', ' ', 'g'))
    );

-- A name of only (Unicode) whitespace normalizes to ''.
ALTER TABLE catalog_entries
    DROP CONSTRAINT catalog_entries_name_not_blank,
    ADD CONSTRAINT catalog_entries_name_not_blank CHECK (name_key <> '');

-- 2. Search filters on the normalized keys (LIKE '%…%'), so the trigram
--    indexes belong on them. The 0002 index on name stays for the
--    categories-style similarity() lookups; the 0005 identifier index was
--    never used.
DROP INDEX IF EXISTS catalog_entries_identifier_trgm;
CREATE INDEX catalog_entries_name_key_trgm       ON catalog_entries USING GIN (name_key gin_trgm_ops);
CREATE INDEX catalog_entries_identifier_key_trgm ON catalog_entries USING GIN (identifier_key gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS catalog_entries_identifier_key_trgm;
DROP INDEX IF EXISTS catalog_entries_name_key_trgm;
CREATE INDEX catalog_entries_identifier_trgm ON catalog_entries USING GIN (identifier gin_trgm_ops);
ALTER TABLE catalog_entries
    DROP CONSTRAINT catalog_entries_name_not_blank,
    ADD CONSTRAINT catalog_entries_name_not_blank CHECK (btrim(name) <> '');
ALTER TABLE catalog_entries
    ALTER COLUMN name_key SET EXPRESSION AS (lower(regexp_replace(btrim(name), '\s+', ' ', 'g'))),
    ALTER COLUMN identifier_key SET EXPRESSION AS (lower(regexp_replace(btrim(coalesce(identifier, '')), '\s+', ' ', 'g')));
-- +goose StatementEnd
