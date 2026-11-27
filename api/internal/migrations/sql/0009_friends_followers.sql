-- +goose Up
-- +goose StatementBegin

-- Friends and followers (ADR 0014). The privacy rule (api/internal/privacy,
-- ADR 0009) reads these through store.Relationships.Between.
--
-- friendships: mutual. One row per unordered pair, stored lowest id first
-- (user_a < user_b), so "A and B" can never exist twice in either order and
-- two people requesting each other at the same moment collide on the
-- primary key instead of creating two requests (the store turns that
-- collision into an accepted friendship). requested_by says who asked.
--   pending   requested_by asked; the other person hasn't answered
--   accepted  friends
-- Declining, cancelling, and unfriending delete the row, so either person
-- can ask again later.
CREATE TABLE friendships (
    user_a       uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b       uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_by uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       text        NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending', 'accepted')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    accepted_at  timestamptz,
    PRIMARY KEY (user_a, user_b),
    CONSTRAINT friendships_ordered_pair CHECK (user_a < user_b),
    CONSTRAINT friendships_requester_in_pair CHECK (requested_by IN (user_a, user_b)),
    CONSTRAINT friendships_accepted_at CHECK ((status = 'accepted') = (accepted_at IS NOT NULL))
);
-- The primary key serves lookups by user_a; this serves user_b.
CREATE INDEX friendships_user_b_idx ON friendships (user_b, user_a);

-- follows: one-way. follower_id asked to follow followee_id; every follow
-- needs the followee's approval (owner decision).
--   pending   waiting for the followee
--   approved  following
-- Denying, cancelling, unfollowing, and removing a follower delete the row.
CREATE TABLE follows (
    follower_id uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id uuid        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      text        NOT NULL DEFAULT 'pending'
                              CHECK (status IN ('pending', 'approved')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    approved_at timestamptz,
    PRIMARY KEY (follower_id, followee_id),
    CONSTRAINT follows_not_self CHECK (follower_id <> followee_id),
    CONSTRAINT follows_approved_at CHECK ((status = 'approved') = (approved_at IS NOT NULL))
);
-- The primary key serves "who do I follow"; this serves "who follows me".
CREATE INDEX follows_followee_idx ON follows (followee_id, status, follower_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS follows;
DROP TABLE IF EXISTS friendships;
-- +goose StatementEnd
