# Review notes: seed-sql-files

Decisions made without the owner, for review.

## Decisions

- **Seeds re-run on every API start, with no tracking table.** Why: the files are idempotent and small, and this keeps local, CI, and production converged with no extra deploy step (like Flyway's repeatable `R__` migrations). Alternative: a `seed_runs` table that records applied files by checksum, or a separate `seed` CLI command run at deploy time.
- **Reference rows use `ON CONFLICT (slug) DO UPDATE ... WHERE ... IS DISTINCT FROM`.** Why: the SQL file is the source of truth for curated data, as the JSON loader was; the `WHERE` makes a no-change re-run write nothing. The idempotency test enforces this for every table by comparing row counts and `xmin` across two runs, so future seed files must follow the same pattern. Alternative: `DO NOTHING`, which would never push schema edits to existing databases.
- **One transaction for the whole seed run.** Why: a broken file leaves the database untouched and the API exits, instead of half-seeding. Alternative: one transaction per file.
- **Dev seeds are allowlisted, and a refusal is a hard error.** With `SEED_DEV_DATA=true`, the API exits unless `APP_ENV` is exactly `development` or `test` (so `prod`, `staging`, `Production`, and empty all refuse), matching the existing `SESSION_SECRET` handling. Alternative: a `production` denylist, or log and skip the dev set.
- **`SEED_DEV_DATA` parses with `strconv.ParseBool`**; anything unparsable (for example `yes`) counts as false and logs a startup warning.
- **Removing a row from a seed file doesn't delete it.** Deletions need a goose migration so collections that reference a category are handled deliberately.
- **Dev set ships as a comment-only placeholder** (`sql/dev/0000_placeholder.sql`) documenting the rules; later PRs add the persona users.
- **Category SQL is one multi-row `INSERT`** in `0001_categories.sql` rather than one file per category. Why: eight rows, one-file edits, same shape as the old JSON. Alternative: one file per category once there are many.
- **The JSON-parity test was temporary.** It proved the SQL matched `categories.json` (commit "seed categories from SQL"), then was replaced by explicit expectations when the JSON was deleted.
- **Added `scripts/reset-db.sh`** (drop and recreate the `collection` database, restart the API, which migrates and seeds) and `Documentation/seed-data.md`. The script drops the database rather than the volume, so it doesn't touch the Go module or node_modules volumes; the doc also covers dropping the volume.
- **CI `pull_request` runs for any base branch.** `push` stays limited to `main`, and the docs-only `paths-ignore` is unchanged.

## Not finished / not verified

- `scripts/reset-db.sh` was not run end-to-end against the local `collection` database (the session's permission guard blocked the destructive drop). Each step was verified separately on a scratch database: migrations through the API, then both seed sets applied twice with `psql --single-transaction`, ending with 8 categories.
- **Resolved 2026-11-27:** the owner ran `scripts/reset-db.sh` end-to-end after #23 merged; it rebuilt the database to the latest migration and applied all seed files.
