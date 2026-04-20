# Collection Tracker

A mobile-first web app for tracking collections of anything — Lego sets, Funko Pops, coins, stamps, plants, whatever. Browse categories without an account; sign in with Google to start saving.

## Stack

- **Frontend:** Nuxt 3 (Vue) + PWA, in `web/`
- **Backend:** Go (chi + pgx), in `api/`
- **Database:** Postgres 16 (JSONB for per-category attributes)
- **Auth:** Google OAuth 2.0
- **Local dev:** Docker Compose
- **CI/CD:** GitHub Actions
- **Future hosting:** AWS (deferred)

See `docs/requirements.md` for the living feature list and `docs/testing.md` for the testing strategy.

## Prerequisites

- Docker + Docker Compose
- Node 18.12+ and pnpm 9+ (for running the Nuxt dev server directly if you prefer)
- (Optional) Go 1.25+ if you want to run API tests outside Docker

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
# Go
(cd api && go test -race ./...)
(cd api && go test -tags=integration ./...)

# Web
(cd web && pnpm test)
(cd web && pnpm test:e2e)
```
