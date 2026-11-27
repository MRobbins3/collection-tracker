-- Dev seed: a small sample of the global catalog (ADR 0004), so category
-- pages, the add-item autocomplete, and "link to the catalog" have real
-- entries to show locally and in CI.
--
-- This is a SAMPLE, not the real dataset: the owner has a specific catalog
-- source in mind, so it lives in the dev set (never applied in production)
-- until that dataset lands. See Documentation/review/global-catalog-review.md.
--
-- Rules (on top of the ones in 0001_dev_users.sql):
--   * Keyed on the natural key (category, normalized name, normalized
--     identifier), the catalog_entries_natural_key unique index. Re-running
--     refreshes description/attributes in place and is a no-op when nothing
--     changed (IS DISTINCT FROM guard), so the seed idempotency test passes.
--   * identifier is copied from the attribute the category names in
--     categories.catalog_identifier (set number, artist...), never typed
--     twice. Coins have no identifier attribute, so the name is the key.
--   * Every entry is source 'seed', status 'approved' (only approved entries
--     are public), approved_by NULL (no admin approved it), and a fixed
--     approved_at so a re-run doesn't rewrite the row.
--   * Attributes must match the category's attribute schema in
--     sql/reference/0001_categories.sql (checked by catalog_integration_test.go).
--     Per-copy details (a coin's year and mint mark, a record's pressing)
--     belong on the user's item, not the catalog entry.
--
-- Sources:
--   Lego sets: Brickset set pages, https://brickset.com/sets/<number>-1
--     (75192, 75313, 10497, 10276, 10294, 21318, 21327, 42115).
--   Vinyl records: each album's Wikipedia article (artist, original release
--     year, original label), e.g. https://en.wikipedia.org/wiki/Abbey_Road.
--   Coins: the US Mint's circulating coin designs,
--     https://www.usmint.gov/learn/coin-and-medal-programs.

INSERT INTO catalog_entries (
    category_id, name, identifier, description, attributes, source, status, approved_at
)
SELECT cat.id,
       e.name,
       e.attributes::jsonb ->> cat.catalog_identifier,
       e.description,
       e.attributes::jsonb,
       'seed',
       'approved',
       '2026-11-27 00:00:00+00'::timestamptz
FROM (VALUES
    -- Lego sets ---------------------------------------------------------------
    ('lego-sets', 'Millennium Falcon', 'Ultimate Collector Series, released 2017.',
     '{"set_number": "75192", "theme": "Star Wars", "piece_count": 7541}'),
    ('lego-sets', 'AT-AT', 'Ultimate Collector Series, released 2021.',
     '{"set_number": "75313", "theme": "Star Wars", "piece_count": 6785}'),
    ('lego-sets', 'Galaxy Explorer', 'Classic Space tribute, released 2022.',
     '{"set_number": "10497", "theme": "Icons", "piece_count": 1254}'),
    ('lego-sets', 'Colosseum', 'Released 2020.',
     '{"set_number": "10276", "theme": "Icons", "piece_count": 9036}'),
    ('lego-sets', 'Titanic', 'Released 2021.',
     '{"set_number": "10294", "theme": "Icons", "piece_count": 9090}'),
    ('lego-sets', 'Tree House', 'Released 2019.',
     '{"set_number": "21318", "theme": "Ideas", "piece_count": 3036}'),
    ('lego-sets', 'Typewriter', 'Released 2021.',
     '{"set_number": "21327", "theme": "Ideas", "piece_count": 2079}'),
    ('lego-sets', 'Lamborghini Sián FKP 37', 'Released 2020.',
     '{"set_number": "42115", "theme": "Technic", "piece_count": 3696}'),

    -- Vinyl records -----------------------------------------------------------
    ('vinyl-records', 'Abbey Road', 'The Beatles'' eleventh studio album.',
     '{"artist": "The Beatles", "album": "Abbey Road", "year": 1969, "label": "Apple Records"}'),
    ('vinyl-records', 'Rumours', 'Fleetwood Mac''s eleventh studio album.',
     '{"artist": "Fleetwood Mac", "album": "Rumours", "year": 1977, "label": "Warner Bros. Records"}'),
    ('vinyl-records', 'The Dark Side of the Moon', 'Pink Floyd''s eighth studio album.',
     '{"artist": "Pink Floyd", "album": "The Dark Side of the Moon", "year": 1973, "label": "Harvest Records"}'),
    ('vinyl-records', 'Kind of Blue', 'Miles Davis''s modal jazz landmark.',
     '{"artist": "Miles Davis", "album": "Kind of Blue", "year": 1959, "label": "Columbia Records"}'),
    ('vinyl-records', 'Thriller', 'Michael Jackson''s sixth studio album.',
     '{"artist": "Michael Jackson", "album": "Thriller", "year": 1982, "label": "Epic Records"}'),
    ('vinyl-records', 'Back in Black', 'AC/DC''s seventh studio album.',
     '{"artist": "AC/DC", "album": "Back in Black", "year": 1980, "label": "Atlantic Records"}'),

    -- Coins (US circulating designs) -------------------------------------------
    ('coins', 'Lincoln Cent', 'US one-cent coin, Lincoln portrait since 1909.',
     '{"country": "United States", "denomination": "1 cent"}'),
    ('coins', 'Jefferson Nickel', 'US five-cent coin, Jefferson portrait since 1938.',
     '{"country": "United States", "denomination": "5 cents"}'),
    ('coins', 'Roosevelt Dime', 'US ten-cent coin, Roosevelt portrait since 1946.',
     '{"country": "United States", "denomination": "10 cents"}'),
    ('coins', 'Washington Quarter', 'US quarter dollar, Washington portrait since 1932.',
     '{"country": "United States", "denomination": "25 cents"}'),
    ('coins', 'Kennedy Half Dollar', 'US half dollar, Kennedy portrait since 1964.',
     '{"country": "United States", "denomination": "Half dollar"}'),
    ('coins', 'Sacagawea Dollar', 'US golden dollar, Sacagawea portrait since 2000.',
     '{"country": "United States", "denomination": "Dollar"}')
) AS e (category_slug, name, description, attributes)
JOIN categories cat ON cat.slug = e.category_slug
-- The natural key is a partial unique index over live entries (0007).
ON CONFLICT (category_id, name_key, identifier_key) WHERE status IN ('pending', 'approved') DO UPDATE SET
    description = EXCLUDED.description,
    attributes  = EXCLUDED.attributes,
    source      = EXCLUDED.source,
    status      = EXCLUDED.status,
    approved_at = EXCLUDED.approved_at,
    updated_at  = now()
WHERE (catalog_entries.description, catalog_entries.attributes, catalog_entries.source,
       catalog_entries.status, catalog_entries.approved_at)
      IS DISTINCT FROM (EXCLUDED.description, EXCLUDED.attributes, EXCLUDED.source,
                        EXCLUDED.status, EXCLUDED.approved_at);
