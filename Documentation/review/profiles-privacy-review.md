# Review notes: profiles-privacy

These are decisions made without the owner, for review. The branch covers profiles (handle, display name, bio, initials avatar), privacy settings with the server-side rule, public profile pages, and account deletion. ADR 0009 (Proposed) has the full model.

## Privacy model

- **Visibility levels are the owner's assumed shape**: Public / Followers & friends / Friends only / Private, plus "accept friend requests" and "accept follow requests". They're account-level, not per collection. A per-collection override can be added later as another input to the same rule.
- **Default visibility: Friends only** for new and backfilled accounts. Strangers see none of a new user's collections, but the profile card stays findable so friend requests can reach them. Backfilled users weren't visible to anyone before, so the default widens nothing. Alternatives: Private (also hides the card, which blocks friend requests) or Public (exposes possibly valuable possessions without consent).
- **Both request toggles default on.** Every follow and friend request still needs approval (owner decision), so "on" exposes nothing by itself. PR 9 enforces the toggles.
- **The profile card is visible at every level except Private.** Private hides the card from strangers (404) but not from existing followers or friends. That only takes effect once PR 9 exists. Alternative: a separate "profile visibility" control. Rejected as an extra setting nobody asked for.
- **Hidden profiles return 404 with exactly the same body as an unknown handle** (`{"error":"profile not found"}`), extending the 404-not-403 convention. The page says "There's no profile here, or its owner keeps it private." A hidden or someone-else's collection is also 404.
- **Friend levels are inert until PR 9.** `store.Relationships.Between` returns only Self or None today. Followers & friends and Friends only therefore behave like Private for collections, but the card is still shown. PR 9 changes only that function. Handler tests already prove that a fake Follower or Friend unlocks the right levels.
- **When someone is both a follower and a friend, Friend wins** (the closer tie).
- **Unknown visibility or relationship values fail closed** (only the owner sees anything).
- **Visitors see the owner's visibility level** (e.g. "Only Noah Williams's friends can see their collections.") so they know why the collections are hidden. Alternative: a generic "collections are hidden". The level isn't very sensitive, and showing it helps people decide whether to send a request.

## Handles

- **Rules:** 3–30 characters of lowercase letters, digits, and underscores, starting with a letter. No dots or hyphens, to avoid look-alikes (`liam.j` vs `liam_j`) and file-extension confusion. Stored lowercase (CHECK constraint), so the plain unique constraint is case-insensitive. `citext` wasn't needed. Input is trimmed, one leading `@` is dropped, and the rest is lowercased.
- **Reserved words** (`admin`, `me`, `settings`, `api`, `auth`, `u`, `my`, `support`, `moderator`, …) are listed in `api/internal/profile`. The migration has a frozen copy, used only for the one-time backfill. The web doesn't duplicate the list: the live availability check reports "reserved".
- **Backfill and sign-up derive the handle only from the display name** (already public). When the name yields nothing usable (e.g. a non-Latin script, or under 3 characters), the handle is `collector` plus a suffix. It never uses the email: handles are published at `/u/<handle>`, and email local parts often contain birth years or work patterns. Collisions and reserved words get `2`, `3`, …, oldest accounts first. Accents are folded the same way in Go (NFD) and SQL (`translate` of the NFD-decomposable Latin letters). `ø` and `ł` don't decompose, so both layers drop them.
- **The availability check leaks handle existence, including for Private profiles.** Unique handles make this unavoidable: any signed-in user can learn `@x` is taken, from the check or a 409 on save. That's how they learn a Private account exists, though they never see its card or collections. It's reduced by requiring sign-in and a per-user, in-memory token bucket on the check (20 quick checks, then one every 3 s; 429 with a friendly message). The bucket is per API process, and the 409 save path isn't limited. Alternative: drop the live check and report "taken" only on save. That's worse UX, and the 409 leaks the same fact.
- **Taken handle on save → 409** with the same `{error, fields}` body as the 400 validation errors.
- **Dev personas' handles are their dev-login keys** (`liam`, `emma`, …), so `/u/liam` works.

## Profile fields

- **Display name: 1–50 characters**, whitespace collapsed. **Sign-in no longer overwrites it with Google's name**, because the user can edit it now. Email still refreshes on every sign-in. Alternative: keep syncing from Google until the user first edits. Rejected as more state for little value.
- **Title Case is suggested only when the name is entirely lowercase** ("Use “Liam Johnson”" link). Anything with a capital letter is treated as deliberate. It's never forced.
- **Bio: 0–160 characters, one line** (newlines collapse to spaces). Alternative: multi-line bios.
- **Avatar: initials plus a color hashed from the user id** (a stable 10-color palette; white text on the Tailwind -700 shades). Photo uploads stay deferred. The API sends the color name, not a hex value, so the palette can be retuned in one place.
- **Profile and settings are one resource** (`GET/PATCH /me/profile`, partial updates) rather than separate `/me/profile` and `/me/settings`. The settings page saves both sections with one button.

## Account deletion

- **`DELETE /me` requires `{"confirm_handle": "<handle>"}`** that matches the current handle (case and `@` ignored), as well as the typed confirmation in the UI. A stray or scripted DELETE can't wipe an account.
- **Hard delete, immediately, with no grace period or export.** Collections and items cascade. Catalog entries the user submitted or approved stay but lose the attribution (`SET NULL`). Alternatives: soft delete with a 30-day undo, or a data export first. Worth considering before real users arrive.
- **Future tables referencing `users` must `ON DELETE CASCADE`** (noted in ADR 0009, CLAUDE.md, and next-steps).

## Public profile pages

- **This partly ships the "Public / shareable collection URLs" item from Explicitly Deferred**, because this PR's scope asked for profile pages that show collections. The deferred line now covers only unlisted share links that ignore account visibility.
- **`/u/<handle>` and `/u/<handle>/c/<id>` load in the browser after auth resolves** (`useAuth().ready`, new). The answer depends on who's asking, and the session cookie never reaches SSR, so this avoids briefly showing the anonymous answer or "not found" to the owner. Trade-off: profile pages aren't server-rendered for SEO.
- **Read-only collection view reuses `ItemCard` with a new `readonly` prop.** The owner sees "Edit in My collections" instead of edit controls.
- **The header shows an avatar account menu** (Your profile, Settings, Sign out) in place of the bare name and Sign-out button. It's a disclosure, not an ARIA `menu`, so links stay links. Existing e2e sign-out steps now open the menu first.

## Testing

- **`POST /auth/dev/disposable`** (registered only when dev auth is on, same JSON + Origin guard as dev login) creates a fresh `dev-disposable:<random>` user and signs in. E2E specs that edit or delete an account use it, so seeded personas never change, and parallel projects don't collide. Disposable users aren't listed in the persona picker. They accumulate in the local database until `scripts/reset-db.sh`. Alternative: re-seed after each spec. That's racy with parallel workers.
- **Persona visibilities cover every level**: Olivia and Liam Public, Emma Private, Noah Friends only, Amelia Followers & friends. Persona bios are invented one-liners, not from a public list.
- **`migrations.UpTo`** was added so the backfill test can build a version-2 database, insert users, then apply 0003, roll it back, and apply it again.

## Review fixes (2026-11-27)

- **Malformed ids** (`/users/x/collections/not-a-uuid`, and the existing `/me/collections/{id}` and item routes) are now 404, not a Postgres 22P02 → 500. The store checks UUID shape in one helper (`validIDs`) before querying, and a real-DB test covers every id-taking collection and item method.
- **Stale sessions after account deletion**: every signed-in route now checks the session's user still exists (one primary-key lookup). If it doesn't, the route answers 401 and clears the cookie, instead of failing on foreign keys on another device.
- **The backfill's `lower()` depends on the database's ctype/collation.** It works for the ASCII and Latin letters that the `translate` list covers. Under an unusual locale, non-ASCII case-folding could differ from Go's `strings.ToLower`. Any character that isn't ASCII after folding becomes `_` anyway, so the worst case is a slightly different backfilled handle, never an invalid one (the CHECK constraint guarantees that).

## Needs owner decision

- **Default visibility Friends only** and **hard delete with no grace period** (both above).
- **`collection_visibility` is shown to every viewer who can see the card**, so visitors learn the owner's level ("Only Noah Williams's friends can see their collections"). It's kept because the message helps people decide whether to send a request. Alternative: a generic "collections are hidden".
- **Handle changes free the old handle immediately**, so someone else could claim it and receive links meant for the previous owner. No cooldown or redirect is built. Options: a 30-day hold on released handles, or redirects from old handles.

## Not done

- Nothing in scope is unfinished. The follower and friend levels are wired but inert until PR 9 by design.
