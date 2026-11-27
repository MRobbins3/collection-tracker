# Collection Tracker — Dependencies and Version Pins

Living document. How tool and dependency versions are pinned, what updates them, and how to bump one without breaking CI. The reasoning behind the policy is in ADR 0006.

## Policy

Every version is pinned exactly. Nothing floats (`latest`, `ubuntu-latest`, `oven/bun:1`, `postgres:16-alpine`), so the environment never changes between commits on its own. Versions change only through a deliberate commit or a Dependabot PR, where CI tests the change in isolation.

## Where versions live

| Tool | Local (`docker-compose.yml`) | CI (`.github/workflows/ci.yml`) | Updated by |
|---|---|---|---|
| Go | `golang:` image | `go-version` (api + e2e jobs) | Dependabot (compose); CI value by hand |
| Bun | `oven/bun:` image | `bun-version` (web + e2e jobs) | Dependabot (compose); CI value by hand |
| Postgres | `postgres:` image | `postgres:` service image (api + e2e jobs) | Dependabot (compose); CI value by hand |
| golangci-lint | — | `version:` on the golangci-lint step | By hand |
| Runner OS | — | `runs-on` (all jobs) | By hand |
| Node (non-Docker tooling) | — | `.nvmrc`, read by `setup-node` | By hand |
| GitHub Actions | — | `uses:` tags | Dependabot |
| Go modules | `api/go.mod` + `go.sum` | same | Dependabot |
| Web packages | `web/package.json` + `bun.lock` | same | Dependabot |

Local and CI must stay on the same Go, Bun, and Postgres versions. Change both in the same commit.

## Dependabot

`.github/dependabot.yml` checks monthly for GitHub Actions, Go modules, Bun packages, and the `docker-compose.yml` images. Minor and patch bumps are grouped into one PR per ecosystem; each major bump gets its own PR so breaking changes stay isolated.

It ignores Postgres major versions (a new major needs a planned data migration; see `next-steps.md`) and `@types/node` majors (they follow `.nvmrc`).

Dependabot can't see the plain version values in `ci.yml`. When it bumps a compose image, push a commit to the same PR that updates the matching CI value from the table above, and merge only when CI is green.

## Bumping by hand

- **golangci-lint.** New releases add rules, so run the new version locally before changing `ci.yml`, and fix or justify any new findings in the same commit. It must be a release built with Go ≥ the `go.mod` target:
  ```sh
  docker run --rm -v "$PWD/api":/app -w /app golangci/golangci-lint:<version> golangci-lint run
  ```
- **Runner OS.** Before moving `runs-on` to a newer Ubuntu, check that the pinned Playwright version supports it (`playwright install --with-deps` fails on unsupported OS releases).
- **Bun.** `bun.lock` is written by whichever Bun ran `bun install`, and CI installs with `--frozen-lockfile`. Keep the compose image and `bun-version` identical so the lockfile CI reads is one it accepts.

Check golangci-lint and the runner OS for new versions every few months, since nothing automated watches them.
