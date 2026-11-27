# Collection Tracker — Next Steps

Living document. The short, scannable "top of the stack": the work you'd start on if you had an hour right now, plus items held for a specific external trigger.

This is NOT the roadmap (that's `requirements.md`) and NOT a changelog. Every follow-up, enhancement, and open owner decision is a GitHub issue (labels `decision`, `follow-up`, `enhancement`, `documentation`); this file links the top few rather than repeating them.

https://github.com/MRobbins3/collection-tracker/issues

**Update cadence:** refresh in the same commit that changes what's queued — whenever a milestone finishes, a bug is found and deferred, or a phase-2 concern graduates to near-term. Prefer pruning over accumulating: anything older than two milestones should either get worked on or get moved out (into `requirements.md`'s deferred / phase-2 section, or dropped entirely). If this list ever grows past ~10 items, it's rotting — trim it.

## Queued work (rough priority order)

1. **Real catalog dataset** (#47, waiting on the owner). Load it as reference data keyed on `catalog_entries_natural_key` (ADR 0011) and retire the 20-entry dev sample in `api/internal/seed/sql/dev/0002_sample_catalog.sql`.
2. **Block users** (#56). Owner decided: the blocked person isn't told, both are hidden from each other, and users see only their own blocked list. Enforce in `store.Relationships.Between` / `api/internal/privacy`.
3. **Submissions as copies + custom category fields** (#51, #54). Owner direction: submitting never takes a group away from its creator; approved-category edits and brand-new fields follow from that.

## Held for external triggers

- **AWS deploy pipeline.** Held until an AWS account is configured. Empty stub YAML rots; wire it only when there's real infra to deploy to. `SESSION_SECRET`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` are the env contract to design around. Behind a load balancer, add one of chi's `middleware.ClientIP*` middlewares that trusts only that proxy if anything needs the client IP (the old `RealIP` was removed because it trusted spoofable headers). The per-user rate limits (`api/internal/server/ratelimit.go`: handle availability checks, friend and follow requests, and people search) are in-memory and per instance: with N API instances a user gets N times the budget, and a restart resets it. Fine for one instance; move them to a shared store (Postgres or Redis) or the load balancer before scaling out.
- **OpenAPI spec + drift check.** Deferred until there's a reason to maintain one — a second client, a public API, or onboarding contributors who'd benefit from the contract. The original testing strategy called for this; it's listed as deferred in `testing.md` and ADR 0001's corrections section.
- **Coverage gate, `@axe-core/playwright`, MSW.** All flagged aspirational in `testing.md`. Ship when a hardening milestone makes sense, or when a specific pain point surfaces. (Prettier is now part of lint hardening, #41.)
- **Real Google sign-in, end to end.** Never tried against live Google (no OAuth credentials yet); only covered by `api/internal/auth/google_test.go` against a fake Google. Before the first deploy, verify consent → callback → session → logout with real credentials on HTTPS (Secure cookies), since #24 moved logout into the session manager and switched the auth plugin to `onNuxtReady`.
