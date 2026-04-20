# Collection Tracker

[![CI](https://github.com/MRobbins3/collection-tracker/actions/workflows/ci.yml/badge.svg)](https://github.com/MRobbins3/collection-tracker/actions/workflows/ci.yml)

A mobile-first web app for tracking collections of anything — Lego sets, Funko Pops, coins, stamps, plants, whatever. Browse categories without an account; sign in with Google to start saving.

## Stack

- **Frontend:** Nuxt 3 (Vue) + PWA, in `web/`
- **Backend:** Go (chi + pgx), in `api/`
- **Database:** Postgres 16 (JSONB for per-category attributes)
- **Auth:** Google OAuth 2.0
- **Local dev:** Docker Compose
- **CI/CD:** GitHub Actions
- **Future hosting:** AWS (deferred)

See `docs/requirements.md` for the living feature list, `docs/testing.md` for the testing strategy, and `CLAUDE.md` for an orientation aimed at any Claude session working in this repo (stack layout, key conventions, test commands).

## Prerequisites

- Docker + Docker Compose — this is the only thing you need to boot the whole stack.
- (Optional) Bun 1.x and Node 24+ if you want to run the web tooling (`bun run test`, `bun run typecheck`) outside Docker. `.nvmrc` at the repo root pins the Node version for `nvm use`.
- (Optional) Go 1.25+ if you want to run API tests outside Docker.

## Getting started

```sh
docker compose up
```

- Web: http://localhost:3000
- API: http://localhost:8080
- Postgres: localhost:5432 (user `collection`, password `collection`, db `collection`)

### Enabling Google sign-in locally

By default, `/auth/google/*` returns 503 until you provide OAuth credentials.
Create a Google OAuth 2.0 Web Client at
<https://console.cloud.google.com/apis/credentials> with authorized redirect URI
`http://localhost:8080/auth/google/callback`, then export the client id/secret
before `docker compose up`:

```sh
export GOOGLE_OAUTH_CLIENT_ID=...
export GOOGLE_OAUTH_CLIENT_SECRET=...
```

## Repo layout

```
web/              Nuxt 3 app
api/              Go service
docs/             Living requirements, testing strategy, ADRs
docker-compose.yml
.github/workflows/
```

## Testing

See `docs/testing.md`. The short version:

```sh
# Go — run inside the api container so Postgres is reachable
docker compose exec api go test -race -count=1 ./...
docker compose exec api go test -race -count=1 -tags=integration ./...
docker compose exec api go vet ./...

# Web — run inside the web container so bun + node_modules + volumes line up
docker compose exec web bun run test        # Vitest (31 tests / 8 files)
docker compose exec web bun run typecheck   # nuxt typecheck (vue-tsc)
docker compose exec web bun run lint        # ESLint via @nuxt/eslint

# Playwright smoke (install browsers once per container)
docker compose exec web bunx playwright install chromium webkit
docker compose exec web bun run test:e2e
```
