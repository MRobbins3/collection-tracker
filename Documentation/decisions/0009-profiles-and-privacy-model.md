# ADR 0009: Profiles, handles, and the privacy model

- **Status:** Proposed
- **Date:** 2026-11-27

## Context

Users need a public identity (a handle and profile page) before friends, followers (PR 9), and collection comparison (PR 10) can exist. Those features all ask the same question: "may this viewer see this owner's collections?" If each feature answers it on its own, the answers drift and private data leaks. The owner decided the shape up front: collection visibility is Public / Followers & friends / Friends only / Private, plus "accept friend requests" and "accept follow requests" toggles, enforced server-side. Following and friending both need approval.

## Decision

### Profile fields (migration 0003, on `users`)

- **`handle`**: public, unique, and always lowercase. It's 3–30 characters of `a-z`, `0-9`, and `_`, and starts with a letter. A list of reserved words (`admin`, `me`, `settings`, `api`, `u`, and others) is refused. Input is normalized (trimmed, one leading `@` dropped, lowercased) before validation, so uniqueness is case-insensitive. A CHECK constraint repeats the format, and `users_handle_key` enforces uniqueness. Rules live in `api/internal/profile` and `web/app/utils/text.ts`.
- **`display_name`** (existing column): 1–50 characters with whitespace collapsed. The user owns it now: Google sign-in refreshes only the email and no longer overwrites the name. The settings page suggests Title Case for all-lowercase names but never forces it.
- **`bio`**: 0–160 characters, kept on one line.
- **Avatar**: no uploads (photo uploads stay deferred). The avatar shows initials plus a palette color derived from the user id (FNV hash), so it survives renames. The API returns the color name and the web maps it to Tailwind classes.
- **Backfill**: a migration step gives every existing user a handle. It's derived only from the display name, which is already public. When the name gives nothing usable (for example a non-Latin script), the handle is `collector` plus a suffix, never anything from the email, because handles are published at `/u/<handle>`. Accents are folded, other characters become `_`, and collisions or reserved words get `2`, `3`, and so on, oldest accounts first. New sign-ups use the same logic in Go (`profile.DeriveHandleBase` + `PickFreeHandle`) and retry on a unique-violation race. The migration is reversible: down drops the new columns.

### Privacy settings

- **`collection_visibility`**: `public | followers_and_friends | friends_only | private` (CHECK constraint).
- **`accept_friend_requests`, `accept_follow_requests`**: booleans, both default `true`. Every follow still needs approval, so accepting requests doesn't expose anything by itself. PR 9 enforces these toggles when requests are created.
- **Default visibility: `friends_only`**, for new and backfilled users. This is privacy by default: strangers see nothing of a new user's collections until the user opts in. The profile card stays findable, so friend requests can reach them. Backfilled users had no visibility to anyone before this change, so the default doesn't widen exposure.

### The rule: one place

`api/internal/privacy` is the only code that interprets visibility:

- `CanViewCollections(visibility, relationship)` returns true for Self always. Public allows everyone, Followers & friends allows Follower or Friend, Friends only allows Friend, and Private allows nobody else.
- `CanViewProfile(visibility, relationship)`: the profile card is visible at every level except Private. A Private profile is visible only to Self and to people who are already a Follower or Friend.
- Unknown values fail closed (only Self sees anything).
- `RelationshipFor(viewerID, ownerID, looked)` makes the owner Self and an anonymous viewer None, whatever the stored lookup says.

The relationship comes from `store.Relationships.Between(viewer, owner)`. It returns **Self or None today**, because friends and followers don't exist yet. PR 9 changes only that function (accepted friendship → Friend, approved follow → Follower; Friend wins when both apply). Every check then honors the follower and friend levels with no handler changes. The handler-side entry point is `resolveView` in `handlers_profile.go`, which every `/users/{handle}` endpoint goes through. The rule has a full matrix table test, and the handler tests prove that a fake Follower or Friend relationship unlocks the right levels.

### API

- `GET /me/profile`, `PATCH /me/profile` (partial; per-field errors `{"error", "fields": {...}}` with 400, or 409 for a taken handle).
- `GET /me/handle-availability?handle=` (signed in only and rate limited per user: 20 quick checks, then one every 3 seconds, else 429; your own handle counts as available).
- `GET /users/{handle}` returns the public card (handle, display name, bio, avatar color; never email, toggles, or ids), `is_self`, `collection_visibility`, `collections_visible`, and `collections` (empty unless visible).
- `GET /users/{handle}/collections/{id}` returns one collection with its items, read-only.
- **Hidden profiles return 404 with the same body as unknown handles** (`{"error":"profile not found"}`), so a stranger can't tell that a Private user exists. This extends the repo's 404-not-403 rule. A collection the viewer can't see is also 404.
- `DELETE /me` deletes the account. Collections and items cascade (`ON DELETE CASCADE`), catalog entries the user submitted or approved stay but lose the attribution (`SET NULL`), and the session cookie is cleared. The body must repeat the handle (`{"confirm_handle": ...}`), and the web asks the user to type it. **Every future table that references `users` must `ON DELETE CASCADE`** (or `SET NULL` for shared content) so deletion stays complete.

### Known residual leaks

- **Handle existence is knowable even for Private profiles.** Handles are unique, so any signed-in user can learn that `@x` is taken, from the availability check or from a 409 when saving. A taken handle can belong to a Private account. This can't be avoided while handles are unique and self-chosen. It's reduced by requiring sign-in and rate limiting the availability check per user. The 404 on `/users/{handle}` still hides the profile itself (name, bio, collections). The 409 path isn't rate limited.
- **Handle changes free the old handle immediately.** Someone else can claim it at once and receive links that pointed at the previous owner. No cooldown or redirect is built (needs an owner decision).
- **`collection_visibility` is returned to every viewer who can see the card**, so a visitor learns the owner's level (e.g. "Friends only"). It's kept for the explanatory message (needs an owner decision).

### Testing aid

`POST /auth/dev/disposable` exists only when dev auth is on (ADR 0008). It creates and signs in a fresh `dev-disposable:` user that is never listed in the persona picker. E2E specs that edit or delete an account use it, so the shared seeded personas stay unchanged.

## Alternatives considered

- **Default `private`.** This is the strictest option, but it also hides the profile card, so nobody could send a friend request. A new user's collections are just as hidden under `friends_only` until they make a friend.
- **Default `public`.** This is the common social-app default, but it exposes collections, which can reveal valuable possessions, without consent.
- **Separate profile visibility setting.** Rejected for now: a fifth control nobody asked for. Private already hides the card from strangers. It can be added later without changing the rule's shape.
- **403 for hidden profiles.** Rejected: it leaks existence.
- **Derive handles from the email** (even as a fallback). Rejected: handles are public, and email local parts often contain real names, birth years, or employer patterns. Non-Latin names get `collector` plus a suffix and can pick a handle in Settings.
- **`citext` for handles.** Rejected: a lowercase CHECK plus normalization at write time gives the same case-insensitivity, and the stored value is the canonical form.
- **Privacy settings in a separate table.** Rejected for now: these are one-to-one with users and always read together. Split them out if settings grow.
- **Collection-level visibility overrides.** Deferred: account-level is what the owner asked for. A per-collection override would become another input to `CanViewCollections`.
