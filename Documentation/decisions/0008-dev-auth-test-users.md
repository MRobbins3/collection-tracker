# ADR 0008: Local-only test-user sign-in (dev auth)

- **Status:** Accepted
- **Date:** 2026-11-27

## Context

Signing in needs real Google OAuth credentials, which most local setups and CI don't have. Without them, nobody can see signed-in pages locally, and Playwright can't test any signed-in flow. Upcoming features (admin moderation, friends, followers, privacy) also need several distinct users to switch between quickly.

## Decision

1. **A second sign-in provider, `auth.Dev`**, that signs the browser in as one of the persona users from the dev seed set. It writes the same session cookie as the Google callback (`Manager.Write`), so `/me`, `auth.Require`, and logout behave identically for both providers.
2. **Off unless explicitly enabled, and never in production.** It's on only with `DEV_AUTH_ENABLED=true`. `config.Validate` makes the API exit at startup if the flag is set and `APP_ENV` isn't exactly `development` or `test`. This is the same allowlist the dev seeds use (`config.DevOnlyAllowed`). When it's off, its routes aren't registered, so they return 404. `docker-compose.yml` and the CI e2e job set the flag; nothing else does.
3. **Endpoints**
   - `GET /auth/providers` returns `{"google": bool, "dev": bool}` so the web knows which buttons to show. This is a separate endpoint so `/me` keeps its anonymous `200 {"user": null}` contract.
   - `GET /auth/dev/personas` lists the `dev:` users with collection and item counts.
   - `POST /auth/dev/login` with `{"persona": "<handle>"}`. An unknown handle returns 404, a malformed one returns 400.
   - `POST /auth/logout` moved from the Google handler to the session manager, since it serves both providers.
4. **Personas are dev seed rows** (`api/internal/seed/sql/dev/0001_dev_users.sql`), keyed on `google_sub = 'dev:<handle>'`. Real Google subject ids are numeric strings, so a `dev:` sub can never match a real account. Each persona covers a scenario: an admin (Olivia, given the role when moderation landed, ADR 0012), a populated collector, a brand-new user, and two users for later friend and follower work.
5. **Login CSRF protection.** The session cookie is already `SameSite=Lax`, which stops cross-site requests from using an existing session. Login CSRF is different: the attacker *sets* a new session, so it needs its own guard. The login endpoint:
   - requires `Content-Type: application/json` and returns 415 otherwise. HTML forms can only send urlencoded, multipart, or `text/plain` bodies, and a cross-origin `fetch` with a JSON body triggers a CORS preflight, which the CORS allowlist rejects;
   - rejects any request whose `Origin` header isn't on the CORS allowlist (403). Requests with no `Origin` (curl, tests) are allowed.
6. **Web.** `AuthMenu` and `SignInPrompt` show a "Sign in as a test user" disclosure, `DevSignIn.vue`, which lists the personas by display name and is labeled as local-only. They hide the Google button only when `/auth/providers` says Google is off and dev auth is on. Providers load in the auth plugin after hydration (`onNuxtReady`). Until then, the components render the Google-only default, so SSR and the first client render match.

## Alternatives considered

- **Mock Google OAuth server** (a fake `/authorize` and `/token` in compose). This exercises more of the real flow, but it adds a service, and switching users means a redirect dance each time. Rejected for now; the Google handler already has unit tests with fake token and userinfo endpoints.
- **Inject a session cookie in Playwright only** (sign a cookie with the test secret). This works for e2e, but it gives the owner nothing to click locally, and it couples tests to the cookie format.
- **Denylist `production`** instead of an allowlist. Rejected for the same reason as the dev seeds: a typo or a new environment name (`prod`, `staging`) would turn it on.
- **Put a persona list in Go code** instead of the seed. Rejected: the users must exist in the database anyway, and later PRs extend them with roles and friendships in SQL.

## Consequences

- Anyone who can reach an API with the flag on can sign in as any persona. That's acceptable only because the flag is allowlisted to development and test, and the API logs a warning at startup when it's on.
- Playwright can now cover signed-in flows (`web/tests/e2e/dev-auth.spec.ts`, using the `signInAsTestUser` helper).
- New persona scenarios are SQL edits to the dev seed. The picker shows whatever `dev:` users exist.
- If `DEV_AUTH_ENABLED=true` but `SEED_DEV_DATA` isn't set, the picker is empty. The API logs a warning at startup, and the picker explains how to fix it.
