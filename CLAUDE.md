# Collection Tracker — Repo Guide for Claude

Mobile-first web app for tracking collections of anything — Lego, Funko, coins, stamps, plants, whatever. Public category browse + Google sign-in + per-user collections + items with category-specific attributes. Installable as a PWA.

## Quick orientation

| Path | What lives there |
|---|---|
| `api/` | Go 1.25 service. chi + pgx + goose migrations + signed-cookie sessions. |
| `web/` | Nuxt 4 + Vue 3 + Tailwind + `@nuxt/icon` (Lucide set) + `@vite-pwa/nuxt`. App code lives under `web/app/` (Nuxt 4 default `srcDir`); `public/`, `tests/`, and config stay at the `web/` root. |
| `Documentation/requirements.md` | **Living** feature list + roadmap table + changelog. Read this first. |
| `Documentation/next-steps.md` | **Living** queued work (priority order) + items held for external triggers. Check this when asked "what's next?" |
| `Documentation/dependencies.md` | **Living** version-pin policy, what Dependabot covers, and how to bump tools CI depends on. |
| `Documentation/testing.md` | **Living** testing strategy — per-language tooling, policies, commands. |
| `Documentation/decisions/` | Architecture Decision Records (ADRs), one numbered file per decision. |
| `Documentation/seed-data.md` | How seed data works (reference vs dev SQL sets), its rules, and how to rebuild the local DB from scratch. |
| `Documentation/categories/` | Attribute-schema conventions. The schemas themselves are seeded from `api/internal/seed/sql/reference/0001_categories.sql`. |
| `api/internal/seed/sql/` | Idempotent SQL seed files: `reference/` (always) and `dev/` (only with `SEED_DEV_DATA=true` and `APP_ENV` `development`/`test`; any other env refuses). ADR 0007. |
| `scripts/reset-db.sh` | Drop and rebuild the local database (migrations + seeds). |
| `docker/web/Dockerfile` | Dev image for the `web` service: Node runtime + Bun package manager (Nuxt must not run on Bun's runtime; ADR 0005). |
| `.github/workflows/ci.yml` | GitHub Actions — api (lint + unit + integration), web (lint + typecheck + vitest), e2e (Playwright smoke). |

## Booting the stack

The one copy of the startup steps (ports, database credentials, and enabling Google sign-in locally) lives in the README under "Getting started". Read it there; don't duplicate it into this file.

## Test commands

The one copy of the lint, test, and typecheck commands lives in `Documentation/testing.md` under each "Running locally" section. Read it there; don't duplicate commands into this file or the README.

## Conventions that matter

- **Testing is first-class**, not an afterthought. Every feature ships with tests at the appropriate layers. Prefer real Postgres over mocks (see `Documentation/testing.md` + ADR 0002).
- **Cross-user access returns 404**, not 403 — hides existence. Enforced at the store layer.
- **`/me` returns `200 {"user": null}`** for anonymous callers, not 401 — app-boot shouldn't splatter red network errors in the browser console.
- **Hydration parity.** SSR HTML and the first client render must match exactly. Two traps we've already hit: (a) `useApi.publicBaseURL` (safe in DOM — same on server and client) vs `fetchBase` (server gets docker hostname, client gets localhost — use this only for the actual fetch call, never for `href`/`src`); (b) async client plugins that mutate reactive state during boot — gate UA-sniffed or browser-only state behind `<ClientOnly>` or defer to `app:mounted`. The install/theme plugins use `app:mounted`; the auth plugin uses `onNuxtReady`, because `app:mounted` fires before an async page (top-level `await useAsyncData`) finishes hydrating, and Vue doesn't patch attribute mismatches. Also: `/me/*` data fetches use `signedInFetchOptions()` (`server: false`, watch `isSignedIn`, skip the request while anonymous), since SSR never has the session cookie and anonymous pages must not log 401s. (c) Text typed into server-rendered inputs before hydration is lost (v-model resets the DOM value), so an SSR'd input that needs JS renders `:disabled="!hydrated"` via `useHydrated()`; e2e then needs no hydration waits.
- **Search boxes** normalize with `normalizeSearchQuery` (`web/app/utils/search.ts`; the API repeats the rule server-side) and debounce with `useDebouncedRef(…, SEARCH_DEBOUNCE_MS)` before the value reaches `useAsyncData`'s `watch`.
- **Test-user sign-in (dev auth, ADR 0008).** With `DEV_AUTH_ENABLED=true` (compose and CI e2e only; the API refuses it outside `APP_ENV` development/test), `/auth/dev/*` signs in as a seeded persona from `api/internal/seed/sql/dev/0001_dev_users.sql`. New scenarios (roles, privacy, friendships) extend those personas in SQL. Playwright signs in with `signInAsTestUser(page, "Liam Johnson")` from `web/tests/e2e/helpers.ts`; prefer that over anonymous-only coverage for signed-in features. Specs that edit or delete an account use `signInAsDisposableUser(page)` (`POST /auth/dev/disposable`, a fresh throwaway user) so the shared personas never change.
- **Privacy rule lives in ONE place (ADR 0009).** Whether a viewer may see a user's profile or collections is decided only by `api/internal/privacy` (`CanViewProfile`, `CanViewCollections`). It takes the owner's visibility and the viewer's relationship from `store.Relationships.Between`, which returns Self/None until friends and followers exist. Handlers go through `resolveView` (`handlers_profile.go`); never compare visibility values anywhere else, and never filter only in the UI. A profile the viewer can't see is 404 with the same body as an unknown handle. New tables referencing `users` must `ON DELETE CASCADE` (or `SET NULL` for shared content) so `DELETE /me` stays complete.
- **Shared text normalizers.** User-typed text goes through `api/internal/textnorm` (`Clean`, `Key`, `Fold`) in Go and `web/app/utils/text.ts` (`collapseWhitespace`, `suggestTitleCase`, `normalizeHandle`) in the web. Handle rules live in `api/internal/profile` and are mirrored for instant feedback in `text.ts`; the API is the authority.
- **Dev-only switches are allowlisted.** Anything dev-only checks `config.DevOnlyAllowed(env)` (exactly `development` or `test`) and refuses to start otherwise.
- **No raw schema keys in UI.** Every technical identifier (`set_number`, slugs, UUIDs) must be paired with a human label. The app is for non-engineers.
- **Two-layer catalog model** (ADR 0004). `items.catalog_entry_id` is nullable; MVP items are all free-form. Don't collapse catalog + item layers; don't remove the `source` / `status` / `approved_by` / `approved_at` columns — they're load-bearing for phase-2 moderation.
- **Seed data is SQL** (ADR 0007, `Documentation/seed-data.md`). Add reference or dev rows as numbered, idempotent files under `api/internal/seed/sql/{reference,dev}/` (`INSERT ... ON CONFLICT` on a natural key, `DO UPDATE` guarded by `IS DISTINCT FROM` so a re-run writes nothing, no `BEGIN`/`COMMIT`, no psql meta-commands). Never seed from Go code or JSON; fake users and other dev-only data go in `dev/`, never `reference/`.
- **Commits:** only when explicitly asked. Milestone commits are pre-authorized by the roadmap. One-line summary + at most 1–2 sentence body; detail goes in PR descriptions. Keep `Documentation/requirements.md` changelog updated in the same commit that changes user-visible behavior.
- **Formatting + linting:** `gofmt` + `goimports` enforced by `golangci-lint` v2 (see `api/.golangci.yml`); `revive`'s `exported` rule is deliberately disabled (internal app, not a library). Web uses `@nuxt/eslint` flat config (see `web/eslint.config.mjs`); vue-tsc is gated via `nuxt typecheck`.
- **CI** (`.github/workflows/ci.yml`) gates merge on `api`, `web`, and `e2e` jobs. Skipped on doc-only commits. If something passes locally but fails in CI, check the common causes listed in `Documentation/testing.md` (CI section).

## When adding to this repo

- Update the roadmap table in `Documentation/requirements.md` if you touch a milestone.
- Update `Documentation/next-steps.md` in the same commit that changes what's queued.
- Add a changelog entry for any user-visible change.
- Add an ADR for any architectural choice worth remembering.
- If introducing a new pattern, document it in this file so the next Claude session picks it up automatically.

## Environment footnotes (things you can't infer from code)

- **Authorship mismatch is intentional.** Commits are authored by `MRobbinsSAI` (local git user); the remote is `github.com/MRobbins3/collection-tracker`. The SSH key on the machine has collaborator access; don't "fix" the mismatch.
- **`.mcp.json` activation.** Context7 is registered in `.mcp.json` but each new Claude Code instance opening this repo has to approve the server on first run. Library-docs lookup becomes available after approval.
