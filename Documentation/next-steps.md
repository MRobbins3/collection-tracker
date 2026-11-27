# Collection Tracker — Next Steps

Living document. The short, scannable "top of the stack": the work you'd start on if you had an hour right now, plus items held for a specific external trigger.

This is NOT the roadmap (that's `requirements.md`) and NOT a changelog.

**Update cadence:** refresh in the same commit that changes what's queued — whenever a milestone finishes, a bug is found and deferred, or a phase-2 concern graduates to near-term. Prefer pruning over accumulating: anything older than two milestones should either get worked on or get moved out (into `requirements.md`'s deferred / phase-2 section, or dropped entirely). If this list ever grows past ~10 items, it's rotting — trim it.

## Queued work (rough priority order)

1. **Root-cause the skipped e2e search-narrow spec.** `tests/e2e/browse-categories.spec.ts > search narrows the list` is `test.skip`'d — in CI, `waitForResponse` for `/categories?q=vinyl` times out (the query-driven refetch never hits the network). Investigation notes in `requirements.md` UX Backlog. Suspected cause: `useAsyncData` with a static key not refetching on `watch` against the CI-hosted dev server. Search is fully covered at the API layer so functional risk is low; fix to make the e2e smoke honest.
2. **MVP feature #6 — text search across a user's own items.** Tracked in `requirements.md` MVP table but not yet scoped to a milestone. Would add `GET /me/items?q=...` (probably backed by the existing GIN trigram index on `items.name`) plus a `/my/search` page or an inline search on `/my`.
3. **Phase 2 — catalog seeding.** ADR 0004 has the full design; schema is already in place (empty `catalog_entries` table + nullable `items.catalog_entry_id` FK + `source` / `status` / `submitted_by` / `approved_by` / `approved_at` columns). First pass could hand-seed ~100 Lego sets and ~200 Funko Pops to prove the catalog-search → pick-from-catalog flow and the "can't find it, enter manually" fallback we already ship for empty results.
4. **Admin role + moderation queue.** Prerequisite for user-submitted catalog entries. `users.role text NULL` migration (nullable; most users stay null), `auth.RequireRole("admin")` middleware on top of `auth.Require`, and admin-only routes for approve/reject. See ADR 0004 "Roles and moderation" — the design is decided, the implementation is greenfield.

## Held for external triggers

- **Postgres 16 → 19.** Dependabot ignores Postgres majors because a new major can't read the old data directory. Upgrade by dumping the local `db-data` volume, bumping the `postgres:` image in `docker-compose.yml` and both CI service images, then restoring. Do it before any hosted deploy so production starts on a current major.

- **AWS deploy pipeline.** Held until an AWS account is configured. Empty stub YAML rots; wire it only when there's real infra to deploy to. `SESSION_SECRET`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` are the env contract to design around.
- **OpenAPI spec + drift check.** Deferred until there's a reason to maintain one — a second client, a public API, or onboarding contributors who'd benefit from the contract. The original testing strategy called for this; it's listed as deferred in `testing.md` and ADR 0001's corrections section.
- **Coverage gate, Prettier, `@axe-core/playwright`, MSW.** All flagged aspirational in `testing.md`. Ship when a hardening milestone makes sense, or when a specific pain point surfaces.
