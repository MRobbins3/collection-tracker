# Seed data

All seed data lives in plain SQL files under `api/internal/seed/sql/`. The API embeds them with `go:embed` and applies them on every start, right after the goose migrations. The design is recorded in ADR 0007.

| Set | Directory | Applied when | Contents |
|---|---|---|---|
| Reference | `api/internal/seed/sql/reference/` | Always, in every environment | The curated categories and their attribute schemas; later, the sample catalog |
| Dev | `api/internal/seed/sql/dev/` | Only with `SEED_DEV_DATA=true`. The API refuses to start with that flag when `APP_ENV=production` | Fake users and sample data for local development and CI (none yet) |

`docker-compose.yml` and the CI e2e job set `SEED_DEV_DATA=true`. Production never does.

## Rules for seed files

- **Idempotent.** Each file runs on every API start and must be safe to re-run after the database is dropped. Use `INSERT ... ON CONFLICT (<natural key>) DO UPDATE` (reference data the file owns) or `DO NOTHING` (rows a user may later edit), keyed on a stable natural key such as a slug, never a generated UUID.
- **Ordered by filename.** Files run in lexical order within a set, reference before dev, so number them (`0001_categories.sql`, `0002_...`). A later file may depend on rows from an earlier one.
- **Plain SQL.** No psql meta-commands and no `BEGIN`/`COMMIT`: the API wraps the whole run in one transaction, so a failing file rolls everything back and the API exits.
- **Additive.** Removing a row from a seed file does not delete it from the database. Write a goose migration for deletions so the rows that reference it are handled deliberately.
- **Cite public sources** in a SQL comment for any realistic sample data.

## Rebuilding the local database from scratch

Drop the database, run the migrations, then the seeds:

```sh
./scripts/reset-db.sh
```

The script drops and recreates the `collection` database inside the running `db` container, then restarts the API, which applies the migrations and seeds before it serves traffic. It destroys every local row.

To drop the whole Postgres volume instead (for example, after a Postgres major-version bump):

```sh
docker compose down
docker volume rm collection-tracker_db-data   # <compose project>_db-data
docker compose up -d
```

## Applying the seeds by hand

With the migrations already applied (start the API once, or run the reset script), the seed files run directly through `psql`. This applies the reference and dev sets in order, in one transaction:

```sh
cat api/internal/seed/sql/reference/*.sql api/internal/seed/sql/dev/*.sql \
  | docker compose exec -T db psql -U collection -d collection -v ON_ERROR_STOP=1 --single-transaction
```

Leave out `api/internal/seed/sql/dev/*.sql` to apply only the reference set.

## Tests

`api/internal/seed/*_integration_test.go` runs against real Postgres (ADR 0002). The tests check that the reference set produces the eight curated categories with labeled schemas, that a second run changes nothing, that dev seeds apply only when enabled, that production refuses them, and that a failing file rolls the whole run back.
