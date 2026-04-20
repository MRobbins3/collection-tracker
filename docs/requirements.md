# Collection Tracker — Requirements

Living document. Status values: `planned` · `in progress` · `shipped` · `deferred`.

Every PR that changes user-visible behavior updates this file in the same commit/PR.

## Vision

A single mobile-first web app where users track collections of *any* kind of thing — Lego sets, Funko Pops, coins, stamps, trading cards, plants, books, vinyl, whatever. Browsing categories is public; saving a collection requires a Google sign-in. Installable as a PWA; native-app wrap via Capacitor is a future option.

## MVP (v0.1)

| # | Feature | Status |
|---|---|---|
| 1 | Public browse + search over a curated seed list of categories | shipped |
| 2 | Google OAuth sign-in; creates/updates user record | planned |
| 3 | Authenticated user can create collections scoped to a category | planned |
| 4 | Add items to a collection with `name`, `quantity`, `condition/variant` + category-specific attributes (JSONB) | planned |
| 5 | Edit / delete items within a collection | planned |
| 6 | Text search across the signed-in user's own items | planned |
| 7 | Mobile-first responsive UI; installable as PWA | planned |

## Explicitly Deferred (post-MVP backlog)

- Photo uploads (requires S3 / AWS object storage work).
- Acquired date / price / provenance tracking.
- User-suggested new categories + moderation flow.
- Public / shareable collection URLs.
- Barcode / UPC scanning for auto-fill.
- Native mobile app build via Capacitor.
- Imports from external catalogs (Brickset, Pop Price Guide, Discogs, etc.).
- Real-time multi-device sync.

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

- 2026-04-19 — Mobile-first category browse UI shipped: `/`, `/categories` (with search), `/categories/:slug` (detail with attribute schema). Tailwind + mobile viewport wired. Vitest component tests + Playwright e2e spec (runs once browsers install).
- 2026-04-19 — Public category endpoints live: `GET /categories` (with `?q=` fuzzy search on name/slug) and `GET /categories/:slug`. Eight curated categories seeded on startup (Books, Coins, Funko Pops, Lego Sets, Plants, Stamps, Trading Cards, Vinyl Records). `/readyz` added; pings DB. API now sends CORS headers for the web origin.
- 2026-04-19 — Initial requirements document created alongside repo scaffold.
