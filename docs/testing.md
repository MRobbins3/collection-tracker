# Collection Tracker — Testing Strategy

Testing is a first-class design concern, not a follow-up. Every feature ships with tests at the appropriate layers, CI gates on them from the first commit, and we prefer real dependencies over mocks wherever reasonable.

## Go API (`api/`)

| Layer | Tooling | What it covers | Pattern |
|---|---|---|---|
| Unit | `testing` stdlib, **table-driven** tests, `github.com/stretchr/testify/require` | Pure functions: validators, JSON Schema checks, URL builders, auth helpers | `cases := []struct{...}; for _, tc := range cases { t.Run(tc.name, ...) }` |
| HTTP handler | `net/http/httptest` + in-process router | Request→response contracts, auth middleware, status codes, serialization | Boot the router in-test, exercise with `httptest.NewRecorder` |
| Integration (DB) | **`testcontainers-go`** with real Postgres per package | sqlc queries, migrations up/down, JSONB validation, transactions | Build tag `//go:build integration`; shared container per package via `TestMain` |
| End-to-end | `docker compose` + test client or Playwright | Full-stack smoke: OAuth callback stub → create collection → add item → retrieve | Smoke only; deep coverage stays in unit + integration |

### Policies

- **No mocking the database.** Integration tests use `testcontainers-go` against real Postgres. Mocked-DB tests hide migration and query bugs.
- **Race detector on.** `go test -race ./...` runs in CI.
- **Coverage floor 70%** on `internal/` packages, enforced via `go test -coverprofile` + a CI gate. Not a ceiling — just a floor that breaks the build on drift.
- **Linting** via `golangci-lint` with `errcheck`, `govet`, `staticcheck`, `revive`, `gosec` enabled.
- **Fuzz tests** for the category attribute JSON validator (`go test -fuzz`).

### Running locally

```sh
cd api
go test -race ./...                 # unit + handler
go test -tags=integration ./...     # integration (needs Docker)
go test -fuzz=FuzzValidate ./internal/category -fuzztime=30s
```

## Nuxt / Vue (`web/`)

| Layer | Tooling | What it covers |
|---|---|---|
| Unit | **Vitest** | Composables (`useApi`, auth state), pure utilities, Pinia stores |
| Component | **Vitest + `@vue/test-utils`**, `@testing-library/vue` queries | Rendered component behavior; accessibility-first selectors; dynamic per-category form rendering |
| Network mocking | **MSW** (Mock Service Worker) | Deterministic API responses in component tests — not hand-rolled fetch stubs |
| E2E | **Playwright** against the docker-compose stack | Anonymous browse + search, Google login (mock OAuth in test env), collection CRUD, item CRUD, PWA manifest |
| Accessibility | **`@axe-core/playwright`** in e2e | Every key page asserted against WCAG AA |
| Mobile viewports | Playwright with iPhone 14 + Pixel 7 profiles | Mobile-first — mobile viewports are the default; desktop is additive |

### Policies

- **ESLint + Prettier**. `pnpm lint` runs in CI.
- **`vue-tsc`** type-checks every PR. Type errors fail CI.
- **No snapshot tests** as a primary strategy; prefer behavior assertions (snapshots rot silently).
- **Storybook explicitly out of scope** for MVP.

### Running locally

```sh
cd web
pnpm test            # Vitest unit + component
pnpm test:e2e        # Playwright e2e against docker-compose
pnpm typecheck
pnpm lint
```

## Shared / Cross-cutting

- **OpenAPI spec** at `api/openapi.yaml` is the contract. CI generates a TypeScript client for Nuxt (`openapi-typescript`); drift between front and back breaks the build.
- **Test data builders** (Go factories, TS fixture helpers) live next to the code they test — no shared `fixtures` god-file.
- **CI (GitHub Actions) gates merge** on: Go unit + integration + race, Go lint, Nuxt unit + component, Nuxt typecheck + lint, Playwright e2e smoke, OpenAPI drift. Every one of these runs on PRs; none are "main-only."
- **This document is living.** Update in the same PR that changes testing conventions.
