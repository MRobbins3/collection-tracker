# ADR 0002: Integration Test Database Strategy

- **Status:** Accepted (with a planned revision when CI lands)
- **Date:** 2026-04-19
- **Supersedes:** none
- **Superseded by:** none (yet)

## Context

The testing strategy (see `docs/testing.md`) calls for real-Postgres integration tests and specifically names `testcontainers-go` as the long-term approach. Two things make testcontainers awkward *right now*:

1. Go isn't installed on the developer's host machine. All Go tooling runs inside the `api` service of `docker-compose`. For testcontainers to work from inside that container, we'd need to either mount the host Docker socket (security hit and environment-specific quirks when the test container reaches its sibling Postgres) or run tests on the host (not an option yet).
2. A Postgres container is already running in our compose stack — spinning up a second one per test package is wasted work at this stage of the project.

## Decision

For local development *and this stage of the project*:

- Integration tests reuse the running `db` service from `docker-compose`.
- Every test provisions a **fresh, uniquely-named database** via a `postgres`-admin connection (`TEST_ADMIN_DATABASE_URL`, defaulting to `postgres://collection:collection@db:5432/postgres?sslmode=disable`).
- After the test, the disposable database is `DROP DATABASE ... WITH (FORCE)`'d. No state leaks into the developer's `collection` database.
- Tests are gated behind the `integration` build tag and invoked with `go test -tags=integration ./...`.

For CI (Milestone 10 and beyond):

- Adopt `testcontainers-go` (or the GitHub Actions `services:` feature — decision deferred) so each package gets an isolated Postgres instance and no runner-shared state.

## Consequences

**Positive**
- Zero new infrastructure: tests run today against the same Postgres the app will use.
- Per-test DBs give real isolation — tests can run in parallel without fighting over table state.
- Migrations are exercised end-to-end on every integration run, which is the primary goal.

**Negative / trade-offs**
- Integration tests are coupled to compose being up, which is fine locally but requires an extra "bring the DB up" step in CI. Revisit with testcontainers.
- Dropping a DB `WITH (FORCE)` is a Postgres 13+ feature — we pin Postgres 16 in compose, so this is fine, but it's worth flagging in any future migration to an older server.
- Admin credentials live in compose env (`collection:collection`). This is dev-only; production will use managed RDS credentials and a restricted app role.

## Revision triggers

Open a new ADR and flip this one to superseded when any of the following is true:
- CI (GitHub Actions) is set up with a different isolation story.
- We want tests to run outside docker-compose without a running Postgres container.
- The number of integration tests grows to the point where spinning DBs serially becomes a noticeable drag.
