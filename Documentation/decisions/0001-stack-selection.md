# ADR 0001: Stack Selection

- **Status:** Accepted (with two in-flight corrections noted below)
- **Date:** 2026-04-19

## Context

Greenfield project: mobile-first web app for tracking arbitrary personal collections. The author is a Vue developer, open to learning a new backend language. Persistent hosting will land on AWS eventually; local dev runs without cloud cost. CI/CD is GitHub Actions. Mobile is the dominant surface, with the option of a native app later.

## Decision

| Layer | Choice |
|---|---|
| Frontend | Nuxt 3 (Vue) + PWA via `@vite-pwa/nuxt` |
| Backend | Go (`chi` router, `pgx` DB driver, `sqlc` for typed queries) |
| Database | Postgres 16 with JSONB for per-category attributes |
| Auth | Google OAuth 2.0 + HTTP-only session cookie |
| Local dev | Docker Compose (db + api + web) |
| CI/CD | GitHub Actions |
| Future hosting | AWS (ECS Fargate or App Runner + RDS + CloudFront/S3) — deferred |

## Consequences

**Positive**
- Nuxt keeps the author productive in Vue while still enabling a PWA and a future Capacitor wrap.
- Go is a genuine learning jump (vs. staying in JS) and compiles to tiny containers that map cleanly to Fargate/App Runner/Lambda.
- JSONB keeps per-category attributes flexible without per-category tables. A single JSON Schema per category validates at the API edge.
- Docker Compose for local dev mirrors the eventual AWS topology (separate service + DB), so deployment is less of a leap.

**Negative / trade-offs**
- Two languages means two test toolchains, two lint configs, and one OpenAPI contract to keep them honest.
- Session cookies require first-party domain setup once the API and web are hosted separately. Revisit if we split domains.
- Go + sqlc + goose adds a small amount of codegen ceremony vs. an ORM.

## Corrections since this ADR was written

- **`sqlc` was not adopted.** The store layer uses plain `pgx` queries with hand-rolled row scanners (`scanCategory`, `scanCollection`, `scanItem`, `scanUser`). Reason: the query surface is small and readable; codegen would be friction without payoff. Revisit if the query set grows (say, beyond 25 distinct queries) or if we want stronger type safety on INSERT/UPDATE column lists.
- **OpenAPI spec was not written.** Handlers were built freehand; there is no `api/openapi.yaml` and no generated TS client. The OpenAPI drift check called out in the testing strategy is deferred until we have a reason to keep a spec up to date (e.g., onboarding a second client or publishing the API).
- **Package manager + runtime moved** from pnpm + Node 20 to Bun + Node 24 pin. See ADR 0005.
