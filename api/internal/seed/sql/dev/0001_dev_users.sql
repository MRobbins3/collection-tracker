-- Dev seed: persona users for local development and CI, plus a few sample
-- collections and items so signed-in pages have something to show.
--
-- Applied only when SEED_DEV_DATA=true, and the API refuses that flag unless
-- APP_ENV is development or test. With DEV_AUTH_ENABLED=true these users
-- appear in the "Sign in as a test user" picker (see ADR 0008).
--
-- Rules for this file (and every file in sql/dev/):
--   * Idempotent: re-runnable after the database is dropped, and on every
--     API start. The seed idempotency test fails CI if a second run inserts,
--     deletes, or rewrites any row.
--   * Persona users are keyed on google_sub = 'dev:<handle>'. Real Google
--     subject ids are numeric strings, so the 'dev:' prefix can never
--     collide with a real account, and the handle is what the dev login
--     endpoint takes.
--   * Collections and items have no natural key, so they use fixed UUIDs
--     (d0000000-...) and ON CONFLICT (id) DO NOTHING. DO NOTHING (not
--     DO UPDATE) means edits made through the app survive a restart; a
--     deleted sample row comes back on the next start.
--   * Emails use example.com (reserved by RFC 2606), so nothing is ever
--     sent to a real inbox.
--
-- Extending: later PRs add roles and friendships for
-- these personas. Add new columns to the users upsert below (and to its
-- IS DISTINCT FROM guard), or add a new 00NN_*.sql file that keys on
-- google_sub, e.g. (SELECT id FROM users WHERE google_sub = 'dev:olivia').
--
-- Personas. First names from the US Social Security Administration's
-- popular baby names list (https://www.ssa.gov/oact/babynames/); surnames
-- from the US Census Bureau's most common surnames
-- (https://www.census.gov/topics/population/genealogy/data/2010_surnames.html).
--
--   handle  | display name     | collections visible to | scenario
--   --------+------------------+------------------------+-------------------------------
--   olivia  | Olivia Martinez  | public                 | admin-to-be (moderates later)
--   liam    | Liam Johnson     | public                 | collector with populated collections
--   emma    | Emma Garcia      | private                | brand-new user, no collections
--   noah    | Noah Williams    | friends_only           | future friend of Liam (one small collection)
--   amelia  | Amelia Brown     | followers_and_friends  | future follower of Liam (one small collection)
--
-- The handle column matches the part of google_sub after 'dev:', so the
-- persona's public profile is /u/<handle>. Visibility covers every privacy
-- level (ADR 0009) so the profile page can be checked against each.

-- Users ----------------------------------------------------------------------

INSERT INTO users (google_sub, email, display_name, handle, bio, collection_visibility) VALUES
    ('dev:olivia', 'olivia.martinez@example.com', 'Olivia Martinez', 'olivia',
     'Keeping the catalog tidy. Collects stamps on the side.', 'public'),
    ('dev:liam',   'liam.johnson@example.com',    'Liam Johnson',    'liam',
     'Lego builder and classic rock vinyl hunter.', 'public'),
    ('dev:emma',   'emma.garcia@example.com',     'Emma Garcia',     'emma',
     'Just getting started.', 'private'),
    ('dev:noah',   'noah.williams@example.com',   'Noah Williams',   'noah',
     'US coins, mostly pocket-change finds.', 'friends_only'),
    ('dev:amelia', 'amelia.brown@example.com',    'Amelia Brown',    'amelia',
     'Houseplants on every windowsill. Cuttings welcome.', 'followers_and_friends')
ON CONFLICT (google_sub) DO UPDATE SET
    email                 = EXCLUDED.email,
    display_name          = EXCLUDED.display_name,
    handle                = EXCLUDED.handle,
    bio                   = EXCLUDED.bio,
    collection_visibility = EXCLUDED.collection_visibility
WHERE (users.email, users.display_name, users.handle, users.bio, users.collection_visibility)
      IS DISTINCT FROM (EXCLUDED.email, EXCLUDED.display_name, EXCLUDED.handle,
                        EXCLUDED.bio, EXCLUDED.collection_visibility);

-- Collections ----------------------------------------------------------------

INSERT INTO collections (id, user_id, category_id, name)
SELECT c.id::uuid, u.id, cat.id, c.name
FROM (VALUES
    ('d0000000-0000-4000-8000-000000000101', 'dev:liam',   'lego-sets',     'Lego Display Shelf'),
    ('d0000000-0000-4000-8000-000000000102', 'dev:liam',   'vinyl-records', 'Classic Rock Vinyl'),
    ('d0000000-0000-4000-8000-000000000103', 'dev:noah',   'coins',         'US Coins'),
    ('d0000000-0000-4000-8000-000000000104', 'dev:amelia', 'plants',        'Living Room Plants')
) AS c (id, google_sub, category_slug, name)
JOIN users u        ON u.google_sub = c.google_sub
JOIN categories cat ON cat.slug = c.category_slug
ON CONFLICT (id) DO NOTHING;

-- Items ----------------------------------------------------------------------
--
-- Lego sets are real sets; names, numbers, themes, and piece counts are from
-- Brickset (https://brickset.com/sets/75192-1, https://brickset.com/sets/10497-1,
-- https://brickset.com/sets/21318-1, https://brickset.com/sets/42115-1).
-- Records are real albums (Wikipedia: "Abbey Road", "Rumours").
-- Coins are real US Mint issues (https://www.usmint.gov/learn/coin-and-medal-programs).
-- Plants are common houseplant species.
-- Attributes must match the category's attribute schema in
-- sql/reference/0001_categories.sql (the API validates app writes, not seeds).

INSERT INTO items (id, collection_id, name, quantity, condition, attributes)
VALUES
    ('d0000000-0000-4000-8000-000000000201', 'd0000000-0000-4000-8000-000000000101',
     'Millennium Falcon', 1, 'New in box',
     '{"set_number": "75192", "theme": "Star Wars", "piece_count": 7541}'),
    ('d0000000-0000-4000-8000-000000000202', 'd0000000-0000-4000-8000-000000000101',
     'Galaxy Explorer', 1, 'Built',
     '{"set_number": "10497", "theme": "Icons", "piece_count": 1254}'),
    ('d0000000-0000-4000-8000-000000000203', 'd0000000-0000-4000-8000-000000000101',
     'Tree House', 1, 'Built',
     '{"set_number": "21318", "theme": "Ideas", "piece_count": 3036}'),
    ('d0000000-0000-4000-8000-000000000204', 'd0000000-0000-4000-8000-000000000101',
     'Lamborghini Sián FKP 37', 1, 'Built',
     '{"set_number": "42115", "theme": "Technic", "piece_count": 3696}'),
    ('d0000000-0000-4000-8000-000000000211', 'd0000000-0000-4000-8000-000000000102',
     'Abbey Road', 1, 'Very good',
     '{"artist": "The Beatles", "album": "Abbey Road", "year": 1969, "label": "Apple Records"}'),
    ('d0000000-0000-4000-8000-000000000212', 'd0000000-0000-4000-8000-000000000102',
     'Rumours', 1, 'Good',
     '{"artist": "Fleetwood Mac", "album": "Rumours", "year": 1977, "label": "Warner Bros. Records"}'),
    ('d0000000-0000-4000-8000-000000000221', 'd0000000-0000-4000-8000-000000000103',
     'Kennedy Half Dollar', 3, 'Circulated',
     '{"year": 1964, "country": "United States", "denomination": "Half dollar", "mint_mark": "D"}'),
    ('d0000000-0000-4000-8000-000000000222', 'd0000000-0000-4000-8000-000000000103',
     'Sacagawea Dollar', 1, 'Uncirculated',
     '{"year": 2000, "country": "United States", "denomination": "Dollar", "mint_mark": "P"}'),
    ('d0000000-0000-4000-8000-000000000231', 'd0000000-0000-4000-8000-000000000104',
     'Monstera', 1, NULL,
     '{"species": "Monstera deliciosa", "pot_size": "10 inch", "acquired_from": "Local nursery"}'),
    ('d0000000-0000-4000-8000-000000000232', 'd0000000-0000-4000-8000-000000000104',
     'Golden Pothos', 2, NULL,
     '{"species": "Epipremnum aureum", "pot_size": "6 inch", "acquired_from": "Cutting from a friend"}')
ON CONFLICT (id) DO NOTHING;
