# Review notes: dev-auth

Decisions made without the owner, for review. ADR 0008 has the design.

## Decisions

- **The API exits at startup if `DEV_AUTH_ENABLED=true` and `APP_ENV` isn't `development` or `test`**, rather than ignoring the flag. Why: this matches the dev-seed refusal and `SESSION_SECRET` handling, and a hard failure can't go unnoticed. Alternative: log and keep running with dev auth off.
- **The allowlist moved to `config.DevOnlyAllowed`**, and `seed.DevSeedsAllowed` now delegates to it, so dev seeds and dev auth can't drift apart.
- **Endpoint names:** `GET /auth/providers`, `GET /auth/dev/personas`, and `POST /auth/dev/login` with `{"persona": "<handle>"}`. When dev auth is off, the routes aren't registered (404), not 503 like unconfigured Google. Why: the brief asked for "don't exist", and 404 reveals nothing.
- **`POST /auth/logout` moved from `auth.Google` to `auth.Manager`.** Why: it only clears the cookie and serves both providers. The route is now registered whenever sessions are configured, not only when Google is.
- **Login CSRF protection: require a JSON content type (415 otherwise) and check `Origin` against the CORS allowlist when the header is present (403).** Why: `SameSite=Lax` doesn't stop login CSRF, because the attacker sets a new cookie. Forms can't send JSON, and a cross-origin JSON `fetch` needs a preflight that CORS rejects. The Origin check is defense in depth. Requests without `Origin` are allowed so curl and tests work. Alternative: a CSRF token, which needs a token round trip for a dev-only endpoint.
- **Personas are identified by handle (`dev:<handle>` google_sub), not by user id.** Handles stay the same after the database is dropped and recreated, so e2e and later seed files can refer to them. Handles must match `^[a-z0-9-]{1,32}$`. They appear only in `data-testid`s and request bodies, never as visible text.
- **The picker shows display name plus "N collections · N items"**, computed live, rather than a hand-written blurb per persona. Why: there's no column for a blurb, and counts stay true as people use the app. Later PRs can show roles once they exist.
- **Personas:** Olivia Martinez (admin-to-be, empty), Liam Johnson (Lego Display Shelf with 4 real sets, Classic Rock Vinyl with 2 albums), Emma Garcia (new, empty), Noah Williams (future friend of Liam; US Coins), and Amelia Brown (future follower of Liam; Living Room Plants). First names come from the SSA baby-names list and surnames from the Census surname list, both cited in the SQL. Emails use `example.com`.
- **Sample collections and items use fixed UUIDs (`d0000000-...`) with `ON CONFLICT (id) DO NOTHING`.** Why: those tables have no natural key. `DO NOTHING` (not `DO UPDATE`) means edits made in the app survive restarts. A sample row someone deletes comes back on the next start. Alternative: add natural-key unique constraints (for example on user + collection name), which is a schema decision for the owner.
- **The Lego piece counts and themes come from Brickset**, as I remember those pages (75192 Millennium Falcon 7541, 10497 Galaxy Explorer 1254, 21318 Tree House 3036, 42115 Lamborghini Sián 3696). I didn't fetch them during the run, so they're worth a glance.
- **A seed test validates every seeded item's attributes against its category schema**, because seeds bypass the API's validation.
- **Web button rules:** until `/auth/providers` loads, and when it fails, the UI shows the old Google-only button. Google is hidden only when providers report `google: false, dev: true`. When neither provider is available, the Google button stays (unchanged behavior). The header button reads "Test sign-in" (short enough for a phone header), and the sign-in prompt reads "Sign in as a test user". Both use amber styling so they don't look like the real sign-in.
- **The picker is a disclosure (button with `aria-expanded` plus a labeled region), not a modal dialog.** Escape closes it and returns focus to the button, clicking outside closes it, and focus moves to the first persona when it opens. Alternative: native `<dialog>`.
- **After a test-user sign-in, the page stays where it is and calls `refreshNuxtData()`.** The Google flow, by contrast, redirects to `/?auth=ok`.
- **The `/my` prompt copy changed** from "Sign in with Google to create collections…" to "Sign in to create collections…", because Google may not be the only option.

## Bugs found and fixed along the way

- **Reloading `/my` while signed in showed "You don't have any collections yet."** `/me/*` `useAsyncData` calls ran during SSR, which never has the session cookie, so they got a 401, and the client didn't refetch after hydration. Fix: `server: false` on `useMyCollections`, `useMyCollection`, and `useCollectionItems`. The signed-in content is client-only anyway, since SSR always renders anonymous.
- **On async pages, the header could hydrate signed-in against anonymous SSR markup**, leaving the anonymous element's attributes behind (missing `title` and `data-testid`). `app:mounted` fires before a page with a top-level `await useAsyncData` finishes hydrating inside `<Suspense>`. Fix: the auth plugin now uses `onNuxtReady`. The theme and install plugins still use `app:mounted`. Their state renders inside `<ClientOnly>`, so they're probably fine, but I didn't audit them.

## Test infrastructure

- **Playwright in the web container needed a way to reach the API.** The browser calls `http://localhost:8080`, which inside the web container isn't the API. Fix: `playwright.config.ts` starts `tests/e2e/api-proxy.mjs`, a TCP forwarder, as a `webServer` only when `E2E_API_PROXY_TARGET` is set (compose sets it). CI and host runs are unaffected.
- **E2E data hygiene:** the collection-flow spec uses a unique name for each project and run, then deletes the collection. If a run fails partway, it can leave an "E2E Coins …" collection on Noah; `./scripts/reset-db.sh` clears it.
- **Converted to e2e:** the signed-in create collection → add item → delete collection flow, which was previously covered only by Go handler and Vitest component tests.

## Not finished / not verified

- Nothing in scope was left unfinished.
- I didn't test against a real Google login to confirm that the logout move and the `onNuxtReady` change behave the same there; no credentials were available.
