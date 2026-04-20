# Collection Tracker — Requirements

Living document. Status values: `planned` · `in progress` · `shipped` · `deferred`.

Every PR that changes user-visible behavior updates this file in the same commit/PR.

## Vision

A single mobile-first web app where users track collections of *any* kind of thing — Lego sets, Funko Pops, coins, stamps, trading cards, plants, books, vinyl, whatever. Browsing categories is public; saving a collection requires a Google sign-in. Installable as a PWA; native-app wrap via Capacitor is a future option.

## MVP (v0.1)

| # | Feature | Status |
|---|---|---|
| 1 | Public browse + search over a curated seed list of categories | shipped |
| 2 | Google OAuth sign-in; creates/updates user record | shipped (set `GOOGLE_OAUTH_CLIENT_ID` / `_SECRET` to enable locally) |
| 3 | Authenticated user can create collections scoped to a category | shipped |
| 4 | Add items to a collection with `name`, `quantity`, `condition/variant` + category-specific attributes (JSONB) | shipped |
| 5 | Edit / delete items within a collection | shipped |
| 6 | Text search across the signed-in user's own items | planned |
| 7 | Mobile-first responsive UI; installable as PWA | planned |

## Milestone Roadmap (after MVP functional slice)

Tracked in order of intended landing. Functional MVP items (#1–#7 above) interleave with these per the original plan (`~/.claude/plans/i-want-to-design-indexed-unicorn.md`).

| Milestone | Title | Status |
|---|---|---|
| 1 | Scaffold monorepo | shipped |
| 2 | Baseline HTTP server + chi + config + logging | shipped |
| 3 | Initial schema + goose migrations | shipped |
| 4 | Seed categories + public browse endpoints | shipped |
| 5 | Public category browse + search UI | shipped |
| 6 | Google OAuth login + session cookie | shipped |
| 7 | Collections CRUD (authed) | shipped |
| 8 | Items CRUD with per-category attributes (human-friendly labels land here) | shipped |
| 9 | **Dark mode** — system-preference-aware theme toggle across web UI | shipped |
| 10 | PWA manifest + installable on mobile | planned |
| 11 | **Toolchain — Bun + Node 24** — switch web package manager from pnpm to Bun; add `.nvmrc` pinning Node 24; update docker-compose web image; validate Vitest + Playwright under bun | planned |
| 12 | GitHub Actions: lint, tests, typecheck, e2e, OpenAPI drift | planned |
| 13 | Living docs sweep — requirements, ADRs, testing doc caught up with shipped behavior | planned |

## Phase 2 — Catalog + community contributions

The long-term product is a **two-layer model** (ADR 0004): a pre-loaded global catalog per category plus user-owned item records that reference it. Captured here as a first-class roadmap so today's work doesn't preclude it.

- **Seeded catalog per category** — curated starter list of well-known items (Lego set numbers, Funko Pop series + numbers, common US/UK/EU coin denominations, etc.). Imported or hand-curated. Users pick from this list to add items without typing.
- **User-submitted catalog entries** — when the thing a user wants isn't in the catalog, they submit it from the item-add flow. New entries are `status = pending` until reviewed.
- **Admin role + moderation queue** — small admin role (most users have none) to approve or reject submitted catalog entries and edit existing ones. `approved_by` / `approved_at` columns on catalog entries are the audit trail (added in Milestone 8 pre-emptively, populated when moderation lands).
- **Catalog supplements / corrections** — users can propose edits to existing catalog entries (missing attribute, wrong year, etc.). Audit-trailed.
- **Catalog-backed autocomplete in the item-add UI** — the name field becomes a search combobox over the catalog, with "can't find it — enter manually" as a first-class escape hatch.
- **Backfill for MVP-era items** — link existing free-form items to catalog entries once matches exist.

Milestone 8 lays the groundwork (nullable `catalog_entry_id` on items, empty `catalog_entries` table with `source`/`status` columns) so the phase-2 work is data/UI population, not a schema migration.

## Explicitly Deferred (post-MVP backlog)

- Photo uploads (requires S3 / AWS object storage work).
- Acquired date / price / provenance tracking.
- User-suggested new categories + moderation flow.
- Public / shareable collection URLs.
- Barcode / UPC scanning for auto-fill.
- Native mobile app build via Capacitor.
- Imports from external catalogs (Brickset, Pop Price Guide, Discogs, etc.).
- Real-time multi-device sync.

## UX Backlog (known-bad surfaces to polish)

_All current entries resolved — add new items here as they come up._

## Known Unknowns (tracked, not blocking)

- Shape of public shareable collections (affects auth model).
- How far we go with curated vs. community-contributed category attribute schemas.
- Postgres full-text search vs. OpenSearch/Meilisearch when the category catalog grows.
- AWS target for Go API: App Runner (simpler) vs. ECS Fargate (more control).

## Seed Categories (initial)

The MVP ships with a curated list:

- Lego Sets
- Funko Pops
- Coins
- Stamps
- Trading Cards
- Plants
- Books
- Vinyl Records

Per-category attribute schemas live in `docs/categories/` and are also stored in the `categories.attribute_schema` JSONB column.

## Changelog

<!-- One line per user-visible change, newest first. Date format YYYY-MM-DD. -->

- 2026-04-19 — Dark mode shipped (Milestone 9). Class-based Tailwind dark variant; `useTheme` composable owns the three-state preference (system / light / dark) with localStorage persistence and `prefers-color-scheme` tracking; a pre-hydration inline script in `<head>` applies the correct theme *before* first paint so there's no flash. `ThemeToggle` in the header cycles states (wrapped in `<ClientOnly>` to avoid SSR/client icon mismatches). Dark variants applied to every surface: landing, header, browse, category detail, /my, /my/:id, item cards, all forms, sign-in prompt, error/empty/success states. 3 new Vitest tests (26 total). Roadmap updated in the same commit to add Milestone 11 "Toolchain — Bun + Node 24" (see ADR 0005) ahead of CI (now M12) and docs sweep (M13).
- 2026-04-19 — Items CRUD shipped (Milestone 8). Migration 0002 adds `catalog_entries` table + nullable `items.catalog_entry_id` FK (ADR 0004 two-layer model; catalog is empty in MVP). Server validates `items.attributes` against the category's JSON Schema (gojsonschema) and returns per-field errors. Public `GET /catalog/entries` endpoint binds the UI to phase-2 shape (returns `[]` today). Category attribute schemas now carry `title` + `description`; UI shows human labels everywhere (closes the Milestone 5 "raw schema keys" backlog). Web: combobox-shaped add item panel, dynamic per-schema form fields, inline edit/delete on item cards, items list on `/my/[id]`. Twelve new Vitest tests (23 total); Go gained integration tests for items + catalog-entry delete preserves items + JSON-Schema validator cases.
- 2026-04-19 — Collections CRUD (authed) shipped. `POST/GET/PATCH/DELETE /me/collections[/:id]` enforce user ownership at the store layer (cross-user reads return 404 to hide existence, not 403). Web `/my` page lists/creates/deletes collections; `/my/[id]` supports rename + delete. Eleven Vitest tests; new handler tests cover auth gate, validation, cross-user isolation, and store-error surfacing. `/me` now returns 200 `{user: null|User}` so app-boot fetches don't emit a red 401 in the browser console (user-reported polish).
- 2026-04-19 — Fixed a Nuxt SSR hydration mismatch: `useApi` was exposing an internal docker hostname that leaked into `href` attributes rendered server-side; split into `publicBaseURL` (safe for DOM) and `fetchBase` (SSR uses the internal URL, client falls back to public).
- 2026-04-19 — Google OAuth sign-in live: `/auth/google/start` (302 → Google), `/auth/google/callback` (upserts user, issues session cookie, redirects to web), `POST /auth/logout`, and `GET /me`. Signed-and-encrypted cookie sessions (ADR 0003); when Google credentials are unset the auth routes return 503 so local dev without creds still works. Web header now shows a "Sign in with Google" link or the signed-in user's name + Sign out. Seven Vitest tests across two components; ten new Go test cases cover session roundtrip, middleware, OAuth start/callback/logout and BestName fallbacks.
- 2026-04-19 — Mobile-first category browse UI shipped: `/`, `/categories` (with search), `/categories/:slug` (detail with attribute schema). Tailwind + mobile viewport wired. Vitest component tests + Playwright e2e spec (runs once browsers install).
- 2026-04-19 — Public category endpoints live: `GET /categories` (with `?q=` fuzzy search on name/slug) and `GET /categories/:slug`. Eight curated categories seeded on startup (Books, Coins, Funko Pops, Lego Sets, Plants, Stamps, Trading Cards, Vinyl Records). `/readyz` added; pings DB. API now sends CORS headers for the web origin.
- 2026-04-19 — Initial requirements document created alongside repo scaffold.
