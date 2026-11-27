-- +goose Up
-- +goose StatementBegin

-- Profiles and privacy settings (ADR 0009).
--
-- handle: public, unique, lowercase username used in /u/<handle>. The format
-- CHECK mirrors profile.ValidateHandle (3–30 chars, a-z 0-9 _, starts with a
-- letter); because it forces lowercase, the plain unique constraint is
-- case-insensitive in effect.
--
-- collection_visibility: who may see the user's collections. The rule that
-- interprets it lives in api/internal/privacy (one place). Default
-- friends_only: strangers see nothing until the user opts in.
ALTER TABLE users
    ADD COLUMN handle                 text,
    ADD COLUMN bio                    text    NOT NULL DEFAULT '',
    ADD COLUMN collection_visibility  text    NOT NULL DEFAULT 'friends_only'
        CONSTRAINT users_collection_visibility_check
        CHECK (collection_visibility IN ('public', 'followers_and_friends', 'friends_only', 'private')),
    ADD COLUMN accept_friend_requests boolean NOT NULL DEFAULT true,
    ADD COLUMN accept_follow_requests boolean NOT NULL DEFAULT true;

-- Backfill a handle for every existing user. Mirrors profile.DeriveHandleBase
-- + PickFreeHandle in Go: use only the display name (already public; never
-- the email, since handles are published), fold accents that NFD
-- decomposes, turn other runs into "_", require 3+ chars (else
-- "collector"), prefix "u_" if it doesn't start with a letter, cap the base
-- at 24 chars, then add 2, 3, … on collision or a reserved word. Oldest
-- accounts go first, so they keep the unsuffixed handle.
DO $backfill$
DECLARE
    r         record;
    base      text;
    candidate text;
    n         int;
    reserved  text[] := ARRAY[
        'about', 'account', 'accounts', 'admin', 'administrator', 'anonymous',
        'api', 'app', 'auth', 'catalog', 'categories', 'category',
        'collection', 'collections', 'collection_tracker', 'contact',
        'delete', 'dev', 'edit', 'everyone', 'follow', 'followers',
        'following', 'friend', 'friends', 'help', 'home', 'login', 'logout',
        'me', 'mod', 'moderator', 'my', 'new', 'null', 'official', 'owner',
        'privacy', 'profile', 'register', 'root', 'security', 'settings',
        'signin', 'signout', 'signup', 'staff', 'support', 'system', 'terms',
        'u', 'undefined', 'user', 'users', 'www'
    ];
BEGIN
    FOR r IN SELECT id, display_name FROM users ORDER BY created_at, id LOOP
        base := NULL;
        candidate := translate(lower(r.display_name),
            'àáâãäåāăąçćčďèéêëēėęěìíîïīįñńňòóôõöōőřśšşťùúûüūůűųýÿžźż',
            'aaaaaaaaacccdeeeeeeeeiiiiiinnnooooooorssstuuuuuuuuyyzzz');
        candidate := btrim(regexp_replace(candidate, '[^a-z0-9]+', '_', 'g'), '_');
        IF length(candidate) >= 3 THEN
            IF candidate !~ '^[a-z]' THEN
                candidate := 'u_' || candidate;
            END IF;
            base := rtrim(left(candidate, 24), '_');
        END IF;
        base := coalesce(base, 'collector');

        candidate := base;
        n := 1;
        WHILE candidate = ANY (reserved)
              OR EXISTS (SELECT 1 FROM users WHERE handle = candidate) LOOP
            n := n + 1;
            candidate := base || n;
        END LOOP;

        UPDATE users SET handle = candidate WHERE id = r.id;
    END LOOP;
END
$backfill$;

ALTER TABLE users
    ALTER COLUMN handle SET NOT NULL,
    ADD CONSTRAINT users_handle_format CHECK (handle ~ '^[a-z][a-z0-9_]{2,29}$'),
    ADD CONSTRAINT users_handle_key UNIQUE (handle);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users
    DROP COLUMN IF EXISTS accept_follow_requests,
    DROP COLUMN IF EXISTS accept_friend_requests,
    DROP COLUMN IF EXISTS collection_visibility,
    DROP COLUMN IF EXISTS bio,
    DROP COLUMN IF EXISTS handle;
-- +goose StatementEnd
