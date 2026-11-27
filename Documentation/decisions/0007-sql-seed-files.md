# ADR 0007: Seed data as idempotent SQL files, split into reference and dev sets

- **Status:** Accepted
- **Date:** 2026-11-27

## Context

Categories were seeded from `api/internal/seed/data/categories.json` by a Go upsert on every API start. Upcoming work needs much more seed data: a sample catalog, plus fake users, friendships, and collections for local development and Playwright. A Go loader per table would grow with every new kind of data. The owner also wants the seeds to be re-runnable by hand after the database is fully dropped.

## Decision

1. Seed data lives in plain SQL files under `api/internal/seed/sql/`, embedded with `go:embed` and applied after the goose migrations on every API start, in one transaction.
2. Two sets: `reference/` (always applied: categories, later the sample catalog) and `dev/` (applied only when `SEED_DEV_DATA=true`). Dev seeds are allowlisted: if they are requested with any `APP_ENV` other than `development` or `test`, the runner returns an error and the API refuses to start.
3. Every file is idempotent (`INSERT ... ON CONFLICT` on a natural key) and runs in filename order. We don't track applied seed files; they re-run each start, like Flyway's repeatable (`R__`) migrations.
4. The files contain no psql meta-commands, so `psql --single-transaction` can run them by hand (see `Documentation/seed-data.md`).

## Alternatives considered

- **Seeds as goose migrations.** Rejected: migrations run once, so editing a category's schema would need a new migration each time, and goose can't apply dev-only data conditionally without a second migration table.
- **Keep JSON plus Go loaders.** Rejected: one loader per table, and the data can't be applied without the Go binary.
- **A separate seed CLI command instead of applying on start.** Rejected for now: applying on start keeps local, CI, and production converged with no extra deploy step. A CLI can wrap `seed.Apply` later if seeding gets slow.

## Consequences

- Adding reference data means editing or adding one SQL file; it reaches every environment on the next deploy.
- Seed files must stay cheap and idempotent because they run on every start.
- Deleting a row from a seed file doesn't delete it from the database; deletions need a migration.
- `ON CONFLICT ... DO UPDATE` makes the file the source of truth for reference rows: hand edits made in the database are overwritten on the next start, as they were with the JSON loader.
