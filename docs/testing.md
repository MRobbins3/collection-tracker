# Collection Tracker — Testing Strategy

Testing is a first-class design concern, not a follow-up. Every feature ships with tests at the appropriate layers, CI gates on them, and we prefer real dependencies over mocks wherever reasonable.

This document describes **what is actually in place** today, with aspirational items clearly flagged.

## Go API (`api/`)

| Layer | Tooling | What it covers | Pattern |
|---|---|---|---|
| Unit | `testing` stdlib, **table-driven** tests, `github.com/stretchr/testify/require` | Pure functions: validators, JSON-Schema checks, config parsing, session encode/decode | `cases := []struct{...}; for _, tc := range cases { t.Run(tc.name, ...) }` |
| HTTP handler | `net/http/httptest` + in-process chi router | Request→response contracts, auth gate, status codes, serialization, CORS | Boot the router via `server.NewRouter`, exercise with `httptest.NewRecorder` |
| OAuth | fake Google token + userinfo endpoints via `httptest.NewServer` | Full callback path: state verify, code exchange, profile fetch, user upsert, session write, redirect | Override `oauth2.Endpoint` + `userinfoURL` on the handler under test |
| Integration (DB) | **Disposable per-test DB** on a shared Postgres (ADR 0002) | Migrations up/down, every store method, JSONB validation, cross-user isolation, CHECK constraints, FK cascades | Build tag `//go:build integration`; `testDBURL()` substitutes the db name into `TEST_ADMIN_DATABASE_URL` so the same code works in docker-compose (host=`db`) and in CI (host=`localhost`) |

### Policies

- **No mocking the database in integration tests.** They run against real Postgres via a disposable per-test database — mocked-DB tests hide migration and query bugs. (ADR 0002 explains why we didn't go with `testcontainers-go`.)
- **Race detector always on.** CI runs `go test -race -count=1 ./...` and `go test -race -count=1 -tags=integration ./...`.
- **Linting** via `golangci-lint` **v2** (installed fresh per run so it's built with Go ≥ 1.25): `errcheck`, `govet`, `staticcheck`, `revive` (with `exported` disabled — internal app, not a library), `gosec` (with G402 excluded; sslmode=disable is dev-only), `ineffassign`, `unused`, `misspell`. Formatters `gofmt` + `goimports` via v2's `formatters` block.
- **Aspirational**: coverage floor, fuzz tests for the JSON-Schema validator. Not gating yet — tracked for a later hardening milestone.

### Running locally

```sh
# inside docker compose
docker compose exec api go test -race -count=1 ./...                  # unit + handler
docker compose exec api go test -race -count=1 -tags=integration ./... # integration
docker compose exec api go vet ./...
```

## Nuxt / Vue (`web/`)

| Layer | Tooling | What it covers |
|---|---|---|
| Component / unit | **Vitest** + **`@vue/test-utils`** under Bun, happy-dom environment | CategoryCard, AuthMenu, NewCollectionForm, AttributeFields, ItemForm, NewItemPanel, ItemCard, InstallPrompt, ThemeToggle — 31 tests across 8 files |
| Composable mocking | Hand-rolled stubs that assign to `globalThis.useXxx` in `beforeEach` | Lets tests drive behavior through real component render paths without pulling in Nuxt's runtime |
| Typecheck | `nuxt typecheck` (vue-tsc) + ambient `shims-vue.d.ts` for SFC resolution in tests | Every PR gated |
| Lint | `@nuxt/eslint` flat config with two local overrides | Every PR gated |
| E2E | **Playwright** against a live stack (API + Nuxt dev server running on the GHA runner) | Anonymous browse + navigate home → categories → detail → back; unknown-slug renders the not-found state. The search-narrow spec is currently `test.skip` — see requirements.md UX Backlog |
| Mobile viewports | Playwright with iPhone 14 (webkit) + Pixel 7 (chromium) profiles | Mobile-first — the two mobile profiles are the default; desktop is additive |
| **Aspirational** | `@axe-core/playwright` for WCAG AA checks; MSW for network mocking; visual regression | Not wired up yet; hand-rolled stubs cover test needs for now |

### Policies

- **ESLint** (`@nuxt/eslint`) gated in CI via `bun run lint`. Prettier not wired up — single-style formatters are deferred.
- **`vue-tsc`** via `nuxt typecheck` gated in CI.
- **No snapshot tests** as a primary strategy; prefer behavior assertions (snapshots rot silently).
- **Storybook explicitly out of scope** for now.

### Running locally

```sh
# inside docker compose
docker compose exec web bun run test         # Vitest unit + component
docker compose exec web bun run typecheck    # vue-tsc via nuxt typecheck
docker compose exec web bun run lint         # ESLint

# Playwright browsers aren't baked into the oven/bun image; install once:
docker compose exec web bunx playwright install chromium webkit
docker compose exec web bun run test:e2e
```

On the host directly: `.nvmrc` pins Node 24 for `nvm use`; then `cd web && bun install && bun run <script>`.

## CI (GitHub Actions)

`.github/workflows/ci.yml` runs on every PR and every push to `main`, skipped on doc-only commits. Three jobs:

| Job | Gates |
|---|---|
| `api` | golangci-lint v2 (latest, so we get a binary built with Go ≥ 1.25); `go test -race` unit + handler; `go test -race -tags=integration` against a `postgres:16-alpine` service container |
| `web` | Bun install with frozen lockfile; ESLint; `nuxt typecheck`; Vitest |
| `e2e` | Runs API + Nuxt dev server in the background on the runner; caches Playwright browsers keyed on `bun.lock`; runs the Playwright smoke against chromium + webkit; uploads `playwright-report/`, `test-results/`, `api.log`, `web.log` on failure |

A `concurrency` group cancels superseded runs for the same ref. Node is installed via `actions/setup-node@v4` with `node-version-file: .nvmrc` so ESLint's Node-shebang-dispatch gets a modern runtime.

**Deferred**:
- **OpenAPI drift** check — the original testing plan called for `api/openapi.yaml` + generated TS client. The spec hasn't been written yet (we built handlers freehand), so the drift check is deferred. Adding one is tracked as a follow-up.
- **Deploy steps** — deliberately not scaffolded until AWS is wired up (empty stub YAML rots).
- **Coverage gate** — same story; tracked for a later hardening milestone.

## Shared / Cross-cutting

- **Test data builders** live next to the code they test — no shared `fixtures` god-file.
- **Cross-user access** always returns `ErrNotFound` / 404 at the store and handler layers, never 403. Integration tests assert this explicitly for every mutating method.
- **Hydration parity** is a recurring bug class. We've hit it twice (`useApi.publicBaseURL` vs `fetchBase`, and the `async` auth plugin). When adding anything UA-sniffed or browser-only to the DOM, gate it in `<ClientOnly>` or defer state changes to `app:mounted`.
- **This document is living.** Update in the same PR that changes testing conventions.
