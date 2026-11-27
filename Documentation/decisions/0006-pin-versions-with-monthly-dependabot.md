# ADR 0006: Pin every tool version exactly; update through monthly Dependabot PRs

- **Status:** Accepted
- **Date:** 2026-11-26

## Context

CI went seven months without a run (April to November 2026). The code didn't change, but the first push afterwards hit three failures and a deprecation warning, all caused by versions that floated:

1. **golangci-lint `version: latest`** resolved to v2.15.0, whose gosec added rule G124 (cookie attributes) and failed the API job on a docs-only commit.
2. **`runs-on: ubuntu-latest`** moved to Ubuntu 26.04, which the locked Playwright 1.59.1 couldn't install browsers on, failing the e2e job.
3. **Node 20 deprecation.** The actions on `@v4`/`@v5`/`@v7` tags targeted Node 20, which GitHub now forces onto Node 24 with a warning.
4. **Bun drift.** Local dev used `oven/bun:1` (floating) while CI pinned 1.2.23, so the lockfile was written by a different Bun than the one validating it.

None of these could have been caught by running checks locally before pushing; each existed only in CI's environment. And because they all piled up over months, they surfaced together, which made the failure harder to read.

## Decision

1. **Pin everything exactly**: the runner OS, golangci-lint, Go, Bun, Node (via `.nvmrc`), the Postgres image, and the `docker-compose.yml` images. Local and CI use the same Go, Bun, and Postgres versions.
2. **Update through Dependabot, monthly**, for GitHub Actions, Go modules, Bun packages, and compose images. Minor and patch bumps are grouped per ecosystem; majors get their own PR.
3. **Bump the rest by hand**: values Dependabot can't see (CI's plain version inputs, golangci-lint, the runner OS) are changed deliberately, following `Documentation/dependencies.md`.

## Alternatives considered

- **Float versions (status quo).** Free upgrades, but breakage arrives unannounced on unrelated commits, often several at once after a quiet period. This is what failed.
- **Pin without Dependabot.** Stable, but versions silently go stale until a security fix or deprecation forces a large, risky jump.
- **Weekly Dependabot.** Faster to pick up fixes, but too much PR noise for a single-maintainer project with irregular activity. Monthly keeps drift small without a constant review queue.

## Consequences

**Positive**
- The environment only changes in a commit that says so, and CI tests each change in isolation.
- Upgrades arrive as small, labelled PRs instead of a pile-up.
- Local and CI can't silently diverge on Go, Bun, or Postgres.

**Negative / trade-offs**
- Values Dependabot can't see must be kept in sync by hand when a compose bump lands. `Documentation/dependencies.md` lists them.
- golangci-lint and the runner OS have no automated watcher; they need an occasional manual check.
- Security patches wait up to a month unless picked up by hand.

## Revision triggers

- A second maintainer or a production deploy raises the cost of a month-long patch delay (consider weekly, or Dependabot security updates on their own schedule).
- Dependabot gains support for workflow version inputs, removing the manual sync step.
