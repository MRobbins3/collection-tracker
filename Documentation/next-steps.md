# Collection Tracker — Next Steps

Living document. The short, scannable "top of the stack": the work you'd start on if you had an hour right now, plus items held for a specific external trigger.

This is NOT the roadmap (that's `requirements.md`) and NOT a changelog.

**Update cadence:** refresh in the same commit that changes what's queued — whenever a milestone finishes, a bug is found and deferred, or a phase-2 concern graduates to near-term. Prefer pruning over accumulating: anything older than two milestones should either get worked on or get moved out (into `requirements.md`'s deferred / phase-2 section, or dropped entirely). If this list ever grows past ~10 items, it's rotting — trim it.

## Queued work (rough priority order)

1. **Phase 2 — catalog seeding.** ADR 0004 has the full design; schema is already in place (empty `catalog_entries` table + nullable `items.catalog_entry_id` FK + `source` / `status` / `submitted_by` / `approved_by` / `approved_at` columns). The dataset (which categories, how many entries, and the source) is decided when this work starts. Seed rows go in a numbered idempotent SQL file under `api/internal/seed/sql/reference/` (ADR 0007). The first seed should prove the catalog-search → pick-from-catalog flow and the "can't find it, enter manually" fallback we already ship for empty results.
2. **Admin role + moderation queue.** The dev persona Olivia Martinez (`dev:olivia`) is the intended first admin for local testing; give her the role in the dev seed when the column lands. Prerequisite for user-submitted catalog entries. `users.role text NULL` migration (nullable; most users stay null), `auth.RequireRole("admin")` middleware on top of `auth.Require`, and admin-only routes for approve/reject. See ADR 0004 "Roles and moderation" — the design is decided, the implementation is greenfield.

## Held for external triggers

- **AWS deploy pipeline.** Held until an AWS account is configured. Empty stub YAML rots; wire it only when there's real infra to deploy to. `SESSION_SECRET`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` are the env contract to design around. Behind a load balancer, add one of chi's `middleware.ClientIP*` middlewares that trusts only that proxy if anything needs the client IP (the old `RealIP` was removed because it trusted spoofable headers).
- **OpenAPI spec + drift check.** Deferred until there's a reason to maintain one — a second client, a public API, or onboarding contributors who'd benefit from the contract. The original testing strategy called for this; it's listed as deferred in `testing.md` and ADR 0001's corrections section.
- **Coverage gate, Prettier, `@axe-core/playwright`, MSW.** All flagged aspirational in `testing.md`. Ship when a hardening milestone makes sense, or when a specific pain point surfaces.
