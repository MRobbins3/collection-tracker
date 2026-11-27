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
- **Backfill and sign-up derive the handle from the display name first** (already public). The email local part is used only if the name yields nothing usable, then `collector`. Why: handles are public, and email local parts often contain birth years or work patterns. Collisions and reserved words get `2`, `3`, …, oldest accounts first. Accents are folded the same way in Go (NFD) and SQL (`translate` of the NFD-decomposable Latin letters). `ø` and `ł` don't decompose, so both layers drop them.
- **The handle availability check requires sign-in** so it can't be used for anonymous enumeration, although public profiles are findable by URL anyway.
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

## Needs owner decision

- None blocking. Worth a look: the **Friends only default** and **hard delete with no grace period** (both above).

## Not done

- Nothing in scope is unfinished. The follower and friend levels are wired but inert until PR 9 by design.
