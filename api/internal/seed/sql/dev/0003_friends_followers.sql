-- Dev seed: friends and followers among the personas (ADR 0014), so every
-- collection visibility level (ADR 0009) can be seen working locally and
-- in CI.
--
--   who                  | tie                          | demonstrates
--   ---------------------+------------------------------+------------------------------------------
--   Liam and Noah        | friends                      | Liam sees Noah's Friends-only collection
--   Liam -> Amelia       | follows (approved)           | Liam sees Amelia's Followers & friends one
--   Amelia -> Liam       | follows (approved)           | Liam's Followers tab has someone in it
--   Amelia -> Emma       | follow request (pending)     | Emma has a request to approve or deny
--   Olivia               | none                         | what a stranger sees of each profile
--
-- Olivia (the admin) has no ties on purpose: specs use her as the
-- stranger, and Emma's request comes from Amelia, not Liam, so Emma's
-- Private profile stays hidden from Liam.
--
-- Rules (on top of the ones in 0001_dev_users.sql):
--   * Keyed on the natural key: the person pair (friendships store it
--     lowest id first, so least()/greatest() here) or the follow
--     direction. ON CONFLICT DO NOTHING, so a relationship changed in the
--     app is left alone, and one removed in the app comes back on the next
--     start, like the sample collections.
--   * Fixed timestamps, so a re-run never rewrites a row.

INSERT INTO friendships (user_a, user_b, requested_by, status, created_at, accepted_at)
SELECT least(r.id, o.id), greatest(r.id, o.id), r.id, f.status,
       f.created_at::timestamptz, f.accepted_at::timestamptz
FROM (VALUES
    ('dev:liam', 'dev:noah', 'accepted', '2026-03-01 12:00:00+00', '2026-03-02 12:00:00+00')
) AS f (requester, other, status, created_at, accepted_at)
JOIN users r ON r.google_sub = f.requester
JOIN users o ON o.google_sub = f.other
ON CONFLICT (user_a, user_b) DO NOTHING;

INSERT INTO follows (follower_id, followee_id, status, created_at, approved_at)
SELECT fr.id, fe.id, f.status, f.created_at::timestamptz, f.approved_at::timestamptz
FROM (VALUES
    ('dev:liam',   'dev:amelia', 'approved', '2026-04-10 12:00:00+00', '2026-04-11 12:00:00+00'),
    ('dev:amelia', 'dev:liam',   'approved', '2026-04-12 12:00:00+00', '2026-04-12 18:00:00+00'),
    ('dev:amelia', 'dev:emma',   'pending',  '2026-11-20 12:00:00+00', NULL)
) AS f (follower, followee, status, created_at, approved_at)
JOIN users fr ON fr.google_sub = f.follower
JOIN users fe ON fe.google_sub = f.followee
ON CONFLICT (follower_id, followee_id) DO NOTHING;
