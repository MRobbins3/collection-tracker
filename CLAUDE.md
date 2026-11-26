# Collection Tracker — Repo Guide for Claude

Mobile-first web app for tracking collections of anything — Lego, Funko, coins, stamps, plants, whatever. Public category browse + Google sign-in + per-user collections + items with category-specific attributes. Installable as a PWA.

## Quick orientation

| Path | What lives there |
|---|---|
| `api/` | Go 1.25 service. chi + pgx + goose migrations + signed-cookie sessions. |
| `web/` | Nuxt 3 + Vue 3 + Tailwind + `@nuxt/icon` (Lucide set) + `@vite-pwa/nuxt`. |
| `Documentation/requirements.md` | **Living** feature list + roadmap table + changelog. Read this first. |
| `Documentation/next-steps.md` | **Living** queued work (priority order) + items held for external triggers. Check this when asked "what's next?" |
| `Documentation/testing.md` | **Living** testing strategy — per-language tooling, policies, commands. |
| `Documentation/decisions/` | ADRs. 0001 stack, 0002 integration-test DB, 0003 session cookies, 0004 catalog two-layer model, 0005 Bun + Node 24. |
| `Documentation/categories/` | Placeholder for future human-readable per-category notes. The actual attribute schemas are seeded from `api/internal/seed/data/categories.json`. |
| `.github/workflows/ci.yml` | GitHub Actions — api (lint + unit + integration), web (lint + typecheck + vitest), e2e (Playwright smoke). |
| `~/.claude/plans/i-want-to-design-indexed-unicorn.md` | The original foundational plan. Snapshot; requirements.md is the moving target. |

## Booting the stack

```sh
docker compose up -d
```

- Web: http://localhost:3000 (Bun + Nuxt dev server)
- API: http://localhost:8080 (Go + chi)
- Postgres: localhost:5432 (user/pass/db all `collection`)

Google OAuth is off by default; export `GOOGLE_OAUTH_CLIENT_ID` + `GOOGLE_OAUTH_CLIENT_SECRET` to turn it on locally. `/auth/google/*` returns 503 until then.

## Test commands

```sh
# Go unit + handler + race detector
docker compose exec api go test -race ./...

# Go integration (disposable per-test Postgres; see ADR 0002)
docker compose exec api go test -race -tags=integration ./...

# Web — Vitest + typecheck + lint under Bun
docker compose exec web bun run test
docker compose exec web bun run typecheck
docker compose exec web bun run lint

# Playwright — browsers aren't baked into oven/bun, install once per container
docker compose exec web bunx playwright install chromium webkit
docker compose exec web bun run test:e2e
```

## Conventions that matter

- **Testing is first-class**, not an afterthought. Every feature ships with tests at the appropriate layers. Prefer real Postgres over mocks (see `Documentation/testing.md` + ADR 0002).
- **Cross-user access returns 404**, not 403 — hides existence. Enforced at the store layer.
- **`/me` returns `200 {"user": null}`** for anonymous callers, not 401 — app-boot shouldn't splatter red network errors in the browser console.
- **Hydration parity.** SSR HTML and the first client render must match exactly. Two traps we've already hit: (a) `useApi.publicBaseURL` (safe in DOM — same on server and client) vs `fetchBase` (server gets docker hostname, client gets localhost — use this only for the actual fetch call, never for `href`/`src`); (b) async client plugins that mutate reactive state during boot — gate UA-sniffed or browser-only state behind `<ClientOnly>` or defer to `app:mounted`. Every auth/install/theme plugin in this repo uses the `app:mounted` pattern for exactly this reason.
- **No raw schema keys in UI.** Every technical identifier (`set_number`, slugs, UUIDs) must be paired with a human label. The app is for non-engineers.
- **Two-layer catalog model** (ADR 0004). `items.catalog_entry_id` is nullable; MVP items are all free-form. Don't collapse catalog + item layers; don't remove the `source` / `status` / `approved_by` / `approved_at` columns — they're load-bearing for phase-2 moderation.
- **Commits:** only when explicitly asked. Milestone commits are pre-authorized by the roadmap. One-line summary + at most 1–2 sentence body; detail goes in PR descriptions. Keep `Documentation/requirements.md` changelog updated in the same commit that changes user-visible behavior.
- **Formatting + linting:** `gofmt` + `goimports` enforced by `golangci-lint` v2 (see `api/.golangci.yml`); `revive`'s `exported` rule is deliberately disabled (internal app, not a library). Web uses `@nuxt/eslint` flat config (see `web/eslint.config.mjs`); vue-tsc is gated via `nuxt typecheck`.
- **CI** (`.github/workflows/ci.yml`) gates merge on `api`, `web`, and `e2e` jobs. Skipped on doc-only commits. Anything that works locally but fails in CI is usually one of: (a) hardcoded `db:5432` where CI needs `localhost`, (b) Node-API compatibility (`Object.groupBy` et al — hence `.nvmrc` + `setup-node` in the workflow), (c) a binary built with an older Go than our `go.mod` targets (golangci-lint's bugbear — pin the action's version carefully).

## When adding to this repo

- Update the roadmap table in `Documentation/requirements.md` if you touch a milestone.
- Update `Documentation/next-steps.md` in the same commit that changes what's queued.
- Add a changelog entry for any user-visible change.
- Add an ADR for any architectural choice worth remembering.
- If introducing a new pattern, document it in this file so the next Claude session picks it up automatically.

## Environment footnotes (things you can't infer from code)

- **Authorship mismatch is intentional.** Commits are authored by `MRobbinsSAI` (local git user); the remote is `github.com/MRobbins3/collection-tracker`. The SSH key on the machine has collaborator access; don't "fix" the mismatch.
- **`.mcp.json` activation.** Context7 is registered in `.mcp.json` but each new Claude Code instance opening this repo has to approve the server on first run. Library-docs lookup becomes available after approval.
- **Auto mode doesn't carry.** If the previous session was in Auto mode, a fresh session won't be — the user will invoke it again if they want it.
- **Google OAuth creds are per-laptop env vars.** There's no committed `.env`. `/auth/google/*` returns a helpful 503 until the two env vars are set. Local dev stays unblocked without them.
