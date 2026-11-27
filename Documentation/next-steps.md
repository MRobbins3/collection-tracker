# Collection Tracker — Next Steps

Living document. The short, scannable "top of the stack": the work you'd start on if you had an hour right now, plus items held for a specific external trigger.

This is NOT the roadmap (that's `requirements.md`) and NOT a changelog.

**Update cadence:** refresh in the same commit that changes what's queued — whenever a milestone finishes, a bug is found and deferred, or a phase-2 concern graduates to near-term. Prefer pruning over accumulating: anything older than two milestones should either get worked on or get moved out (into `requirements.md`'s deferred / phase-2 section, or dropped entirely). If this list ever grows past ~10 items, it's rotting — trim it.

## Queued work (rough priority order)

1. **Root-cause the skipped e2e search-narrow spec.** `tests/e2e/browse-categories.spec.ts > search narrows the list` is `test.skip`'d — in CI, `waitForResponse` for `/categories?q=vinyl` times out (the query-driven refetch never hits the network). Investigation notes in `requirements.md` UX Backlog. Suspected cause: `useAsyncData` with a static key not refetching on `watch` against the CI-hosted dev server. Search is fully covered at the API layer so functional risk is low; fix to make the e2e smoke honest.
2. **MVP feature #6 — text search across a user's own items.** Tracked in `requirements.md` MVP table but not yet scoped to a milestone. Would add `GET /me/items?q=...` (probably backed by the existing GIN trigram index on `items.name`) plus a `/my/search` page or an inline search on `/my`.
3. **Phase 2 — catalog seeding.** ADR 0004 has the full design; schema is already in place (empty `catalog_entries` table + nullable `items.catalog_entry_id` FK + `source` / `status` / `submitted_by` / `approved_by` / `approved_at` columns). The dataset (which categories, how many entries, and the source) is decided when this work starts. Seed rows go in a numbered idempotent SQL file under `api/internal/seed/sql/reference/` (ADR 0007). The first seed should prove the catalog-search → pick-from-catalog flow and the "can't find it, enter manually" fallback we already ship for empty results.
4. **Admin role + moderation queue.** The dev persona Olivia Martinez (`dev:olivia`) is the intended first admin for local testing; give her the role in the dev seed when the column lands. Prerequisite for user-submitted catalog entries. `users.role text NULL` migration (nullable; most users stay null), `auth.RequireRole("admin")` middleware on top of `auth.Require`, and admin-only routes for approve/reject. See ADR 0004 "Roles and moderation" — the design is decided, the implementation is greenfield.

## Held for external triggers

- **AWS deploy pipeline.** Held until an AWS account is configured. Empty stub YAML rots; wire it only when there's real infra to deploy to. `SESSION_SECRET`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` are the env contract to design around. Behind a load balancer, add one of chi's `middleware.ClientIP*` middlewares that trusts only that proxy if anything needs the client IP (the old `RealIP` was removed because it trusted spoofable headers).
- **OpenAPI spec + drift check.** Deferred until there's a reason to maintain one — a second client, a public API, or onboarding contributors who'd benefit from the contract. The original testing strategy called for this; it's listed as deferred in `testing.md` and ADR 0001's corrections section.
- **Coverage gate, Prettier, `@axe-core/playwright`, MSW.** All flagged aspirational in `testing.md`. Ship when a hardening milestone makes sense, or when a specific pain point surfaces.
- **Real Google sign-in, end to end.** Never tried against live Google (no OAuth credentials yet); only covered by `api/internal/auth/google_test.go` against a fake Google. Before the first deploy, verify consent → callback → session → logout with real credentials on HTTPS (Secure cookies), since #24 moved logout into the session manager and switched the auth plugin to `onNuxtReady`.
