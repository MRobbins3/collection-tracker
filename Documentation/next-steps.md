# Collection Tracker — Next Steps

Living document. The short, scannable "top of the stack": the work you'd start on if you had an hour right now, plus items held for a specific external trigger.

This is NOT the roadmap (that's `requirements.md`) and NOT a changelog.

**Update cadence:** refresh in the same commit that changes what's queued — whenever a milestone finishes, a bug is found and deferred, or a phase-2 concern graduates to near-term. Prefer pruning over accumulating: anything older than two milestones should either get worked on or get moved out (into `requirements.md`'s deferred / phase-2 section, or dropped entirely). If this list ever grows past ~10 items, it's rotting — trim it.

## Queued work (rough priority order)

1. **Real catalog dataset (waiting on the owner).** Seeding, browse, autocomplete, and item linking shipped with a 20-entry cited sample in `api/internal/seed/sql/dev/0002_sample_catalog.sql` (ADR 0011). The owner has a specific dataset in mind; when it arrives, load it as reference data (or an import) keyed on the `catalog_entries_natural_key` unique index (live entries only, migration 0007), copy `identifier` from each category's `catalog_identifier` attribute, and retire the dev sample.
2. **Friends and followers.** Profiles, handles, and the privacy rule shipped (ADR 0009); the follower and friend visibility levels are inert until this lands. Implement it in `store.Relationships.Between` (accepted friendship → Friend, approved follow → Follower). Every privacy check then picks it up, with no handler changes. Honor `accept_friend_requests` / `accept_follow_requests` when requests are created. New tables referencing `users` must `ON DELETE CASCADE` so `DELETE /me` stays complete.

3. **Owner decisions on user-built categories** (ADR 0013, see `Documentation/review/user-categories-review.md`): brand-new custom fields beyond the library, and how an approved category changes (a `category_correction` moderation kind for new options/fields is the likely shape).

## Held for external triggers

- **AWS deploy pipeline.** Held until an AWS account is configured. Empty stub YAML rots; wire it only when there's real infra to deploy to. `SESSION_SECRET`, `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` are the env contract to design around. Behind a load balancer, add one of chi's `middleware.ClientIP*` middlewares that trusts only that proxy if anything needs the client IP (the old `RealIP` was removed because it trusted spoofable headers).
- **OpenAPI spec + drift check.** Deferred until there's a reason to maintain one — a second client, a public API, or onboarding contributors who'd benefit from the contract. The original testing strategy called for this; it's listed as deferred in `testing.md` and ADR 0001's corrections section.
- **Coverage gate, Prettier, `@axe-core/playwright`, MSW.** All flagged aspirational in `testing.md`. Ship when a hardening milestone makes sense, or when a specific pain point surfaces.
- **Real Google sign-in, end to end.** Never tried against live Google (no OAuth credentials yet); only covered by `api/internal/auth/google_test.go` against a fake Google. Before the first deploy, verify consent → callback → session → logout with real credentials on HTTPS (Secure cookies), since #24 moved logout into the session manager and switched the auth plugin to `onNuxtReady`.
