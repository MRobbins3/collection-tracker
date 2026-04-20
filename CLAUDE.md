# Collection Tracker — Repo Guide for Claude

Mobile-first web app for tracking collections of anything — Lego, Funko, coins, stamps, plants, whatever. Public category browse + Google sign-in + per-user collections + items with category-specific attributes. Installable as a PWA.

## Quick orientation

| Path | What lives there |
|---|---|
| `api/` | Go 1.25 service. chi + pgx + goose migrations + signed-cookie sessions. |
| `web/` | Nuxt 3 + Vue 3 + Tailwind + `@nuxt/icon` (Lucide set) + `@vite-pwa/nuxt`. |
| `docs/requirements.md` | **Living** feature list + roadmap table + changelog. Read this first. |
| `docs/testing.md` | **Living** testing strategy — per-language tooling, policies, commands. |
| `docs/decisions/` | ADRs. Start at 0001 (stack selection). 0003 = session cookies. 0004 = catalog model. 0005 = Bun + Node 24. |
| `docs/categories/` | Per-category attribute-schema docs. |
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

# Web — Vitest under Bun
docker compose exec web bun run test
docker compose exec web bun run typecheck

# Playwright — requires browsers, deferred to Milestone 12 (CI)
docker compose exec web bunx playwright install chromium && docker compose exec web bun run test:e2e
```

## Conventions that matter

- **Testing is first-class**, not an afterthought. Every feature ships with tests at the appropriate layers. Prefer real Postgres over mocks (see `docs/testing.md` + ADR 0002).
- **Cross-user access returns 404**, not 403 — hides existence. Enforced at the store layer.
- **`/me` returns `200 {"user": null}`** for anonymous callers, not 401 — app-boot shouldn't splatter red network errors in the browser console.
- **`useApi.publicBaseURL` vs `fetchBase`.** Anything that lands in the DOM (href, src) must use `publicBaseURL`. Only server-side fetches use the internal docker hostname. Getting this wrong causes SSR/client hydration mismatches.
- **No raw schema keys in UI.** Every technical identifier (`set_number`, slugs, UUIDs) must be paired with a human label. The app is for non-engineers.
- **Two-layer catalog model** (ADR 0004). `items.catalog_entry_id` is nullable; MVP items are all free-form. Don't collapse catalog + item layers; don't remove the `source` / `status` / `approved_by` / `approved_at` columns — they're load-bearing for phase-2 moderation.
- **Commits:** only when explicitly asked. Milestone commits are pre-authorized by the roadmap. One-line summary + at most 1–2 sentence body; detail goes in PR descriptions. Keep `docs/requirements.md` changelog updated in the same commit that changes user-visible behavior.
- **Formatting:** `gofmt` for Go (no explicit runner). Web lint/format comes in Milestone 12 when CI lands.

## Roadmap snapshot

See `docs/requirements.md` for the canonical table. As of most recent commit: milestones 1–11 shipped (scaffold → baseline API → migrations → seed + browse → UI → OAuth → collections → items → dark mode → PWA → Bun+Node 24). Remaining: **12 GitHub Actions CI**, **13 docs sweep**.

## When adding to this repo

- Update the roadmap table in `docs/requirements.md` if you touch a milestone.
- Add a changelog entry for any user-visible change.
- Add an ADR for any architectural choice worth remembering.
- If introducing a new pattern, document it in this file so the next Claude session picks it up automatically.
