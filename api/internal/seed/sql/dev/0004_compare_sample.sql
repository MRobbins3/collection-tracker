-- Dev seed: a Lego collection for Noah that overlaps Liam's, so "Compare
-- collections" between the two friends shows every bucket (Liam ↔ Noah are
-- friends in 0003_friends_followers.sql; Noah shares with friends only).
--
-- Rules are the ones in 0001_dev_users.sql: fixed UUIDs (d0000000-...),
-- ON CONFLICT (id) DO NOTHING, so a re-run changes nothing and edits made in
-- the app survive a restart.
--
-- What Liam sees comparing with Noah in Lego Sets (Liam's sets are
-- free-form; their names each name exactly one catalog entry):
--   both:        Galaxy Explorer (Noah's copy is free-form, lowercase),
--                Tree House (Noah's copy is linked to the catalog)
--   Liam only:   Millennium Falcon, Lamborghini Sián FKP 37
--   Noah only:   Typewriter (linked), NASA Apollo Saturn V (not in the
--                sample catalog, so matched by name)
--   neither:     AT-AT, Colosseum, Titanic (from 0002_sample_catalog.sql)
--
-- Sources: Brickset set pages, https://brickset.com/sets/<number>-1
-- (21318, 10497, 21327, 92176).

INSERT INTO collections (id, user_id, category_id, name)
SELECT c.id::uuid, u.id, cat.id, c.name
FROM (VALUES
    ('d0000000-0000-4000-8000-000000000105', 'dev:noah', 'lego-sets', 'Lego Shelf')
) AS c (id, google_sub, category_slug, name)
JOIN users u        ON u.google_sub = c.google_sub
JOIN categories cat ON cat.slug = c.category_slug
ON CONFLICT (id) DO NOTHING;

-- catalog_entry_id is looked up by the entry's natural key (category, name,
-- identifier), so it never hard-codes a generated id. NULL = free-form.
INSERT INTO items (id, collection_id, name, quantity, condition, attributes, catalog_entry_id, created_at)
SELECT i.id::uuid, i.collection_id::uuid, i.name, i.quantity, i.condition, i.attributes::jsonb,
       (SELECT ce.id FROM catalog_entries ce
        JOIN categories cat ON cat.id = ce.category_id
        WHERE cat.slug = 'lego-sets' AND ce.status = 'approved'
          AND ce.name_key = lower(i.link_name) AND ce.identifier_key = i.link_set),
       i.created_at::timestamptz
FROM (VALUES
    ('d0000000-0000-4000-8000-000000000241', 'd0000000-0000-4000-8000-000000000105',
     'Tree House', 1, 'Built',
     '{"set_number": "21318", "theme": "Ideas", "piece_count": 3036}',
     'Tree House', '21318', '2026-05-02 12:00:00+00'),
    ('d0000000-0000-4000-8000-000000000242', 'd0000000-0000-4000-8000-000000000105',
     'galaxy explorer', 2, 'Used',
     '{"set_number": "10497", "theme": "Icons", "piece_count": 1254}',
     NULL, NULL, '2026-06-11 12:00:00+00'),
    ('d0000000-0000-4000-8000-000000000243', 'd0000000-0000-4000-8000-000000000105',
     'Typewriter', 1, 'New in box',
     '{"set_number": "21327", "theme": "Ideas", "piece_count": 2079}',
     'Typewriter', '21327', '2026-08-19 12:00:00+00'),
    ('d0000000-0000-4000-8000-000000000244', 'd0000000-0000-4000-8000-000000000105',
     'NASA Apollo Saturn V', 1, 'Built',
     '{"set_number": "92176", "theme": "Ideas", "piece_count": 1969}',
     NULL, NULL, '2026-09-30 12:00:00+00')
) AS i (id, collection_id, name, quantity, condition, attributes, link_name, link_set, created_at)
WHERE EXISTS (SELECT 1 FROM collections WHERE id = i.collection_id::uuid)
ON CONFLICT (id) DO NOTHING;
