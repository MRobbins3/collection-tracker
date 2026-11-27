-- Dev seed set: fake data for local development and CI only.
--
-- Files in this directory run after the reference set, in filename order,
-- only when SEED_DEV_DATA=true. The API refuses to start with that flag
-- when APP_ENV=production. Every file must be idempotent (re-runnable after
-- the database is dropped, and on every API start): use
-- INSERT ... ON CONFLICT ... DO UPDATE / DO NOTHING with stable keys.
--
-- Nothing here yet; persona users and sample collections land in later PRs.
SELECT 1;
