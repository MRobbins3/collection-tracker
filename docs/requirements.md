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
| 4 | Add items to a collection with `name`, `quantity`, `condition/variant` + category-specific attributes (JSONB) | planned |
| 5 | Edit / delete items within a collection | partial — collection rename/delete shipped; item CRUD lands in Milestone 8 |
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
| 8 | Items CRUD with per-category attributes (human-friendly labels land here) | planned |
| 9 | **Dark mode** — system-preference-aware theme toggle across web UI | planned |
| 10 | PWA manifest + installable on mobile | planned |
| 11 | GitHub Actions: lint, tests, typecheck, e2e, OpenAPI drift | planned |
| 12 | Living docs sweep — requirements, ADRs, testing doc caught up with shipped behavior | planned |

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

- **Category-specific fields use raw schema keys.** `/categories/:slug` currently renders property names like `set_number` and `piece_count` straight from the JSON Schema. Non-engineers will find this jargon-y. Fix: extend each category's `attribute_schema` with `title` (human label) and optional `description`/`example`, and have the UI prefer those over the raw key. Tackle alongside Milestone 8 (dynamic per-category item form) so labels land in one sweep across detail page + add/edit form. Flagged by user during Milestone 5 review (2026-04-19).

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

- 2026-04-19 — Collections CRUD (authed) shipped. `POST/GET/PATCH/DELETE /me/collections[/:id]` enforce user ownership at the store layer (cross-user reads return 404 to hide existence, not 403). Web `/my` page lists/creates/deletes collections; `/my/[id]` supports rename + delete. Eleven Vitest tests; new handler tests cover auth gate, validation, cross-user isolation, and store-error surfacing. `/me` now returns 200 `{user: null|User}` so app-boot fetches don't emit a red 401 in the browser console (user-reported polish).
- 2026-04-19 — Fixed a Nuxt SSR hydration mismatch: `useApi` was exposing an internal docker hostname that leaked into `href` attributes rendered server-side; split into `publicBaseURL` (safe for DOM) and `fetchBase` (SSR uses the internal URL, client falls back to public).
- 2026-04-19 — Google OAuth sign-in live: `/auth/google/start` (302 → Google), `/auth/google/callback` (upserts user, issues session cookie, redirects to web), `POST /auth/logout`, and `GET /me`. Signed-and-encrypted cookie sessions (ADR 0003); when Google credentials are unset the auth routes return 503 so local dev without creds still works. Web header now shows a "Sign in with Google" link or the signed-in user's name + Sign out. Seven Vitest tests across two components; ten new Go test cases cover session roundtrip, middleware, OAuth start/callback/logout and BestName fallbacks.
- 2026-04-19 — Mobile-first category browse UI shipped: `/`, `/categories` (with search), `/categories/:slug` (detail with attribute schema). Tailwind + mobile viewport wired. Vitest component tests + Playwright e2e spec (runs once browsers install).
- 2026-04-19 — Public category endpoints live: `GET /categories` (with `?q=` fuzzy search on name/slug) and `GET /categories/:slug`. Eight curated categories seeded on startup (Books, Coins, Funko Pops, Lego Sets, Plants, Stamps, Trading Cards, Vinyl Records). `/readyz` added; pings DB. API now sends CORS headers for the web origin.
- 2026-04-19 — Initial requirements document created alongside repo scaffold.
