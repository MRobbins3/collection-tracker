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

See `Documentation/requirements.md` for the living feature list, `Documentation/testing.md` for the testing strategy, `Documentation/next-steps.md` for what is queued next, and `CLAUDE.md` for an orientation aimed at any Claude session working in this repo (stack layout, key conventions, test commands).

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
Documentation/    Living requirements, testing strategy, ADRs
docker-compose.yml
.github/workflows/
```

## Testing

Lint, test, and typecheck commands for the API, the web app, and the Playwright end-to-end tests are in `Documentation/testing.md`, under each "Running locally" section.
