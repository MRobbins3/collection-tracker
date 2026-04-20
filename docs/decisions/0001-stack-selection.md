# ADR 0001: Stack Selection

- **Status:** Accepted
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
