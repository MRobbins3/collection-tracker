# ADR 0003: Session cookies over JWTs

- **Status:** Accepted
- **Date:** 2026-04-19

## Context

We need to keep users authenticated across requests after they sign in with Google. The two plausible shapes are:

1. **Server-side sessions with a signed cookie** — cookie holds a small opaque/encoded payload (here: the user id), server re-loads anything else from the DB on every request.
2. **JWT access tokens in a cookie or Authorization header** — client carries the claims itself; server verifies signature per request and never hits the DB.

## Decision

Use **signed-and-encrypted session cookies** (`gorilla/securecookie`) carrying only the user id. Look everything else up on each request via `users.GetByID`.

Concrete choices:
- Cookie name: `ct_session`.
- Cookie flags: `HttpOnly`, `SameSite=Lax`, `Secure` only in production. `Lax` survives the redirect bounce from Google back to our callback.
- Lifetime: 30 days rolling (re-sliding TTL can land later if it matters).
- Secret: 32-byte `SESSION_SECRET`. In dev, if unset we generate an ephemeral one and log a warning; in production the service refuses to start without one.
- State for OAuth: short-lived (5 min) `ct_oauth_state` cookie set on `/auth/google/start`, verified and cleared on `/auth/google/callback`.

## Consequences

**Positive**
- **Revocable.** Sign-out clears the cookie. Server-side logout/password-reset equivalents are easy — just invalidate by swapping the secret or gating on a user table flag.
- **Small payload, small blast radius.** Cookie holds only a uuid; no PII in transit.
- **No refresh-token machinery.** JWTs would need a refresh token and a rotation flow to be safe; sessions don't.
- **Trivial integration with Google's redirect flow.** Cookie-based flows pair naturally with a top-level navigation to the OAuth start URL.

**Negative / trade-offs**
- Every authenticated request costs a DB lookup for the user row. Acceptable at our scale; cache behind `singleflight` if it ever isn't.
- Horizontal scaling (multiple API instances) requires either sticky routing or a shared session store. Cookie-only signing sidesteps this for now — the cookie is the session, so any instance can validate it as long as they share the secret.
- Cross-site cookie delivery is strict (`SameSite=Lax`). If we ever want the web app on a different eTLD+1 than the API, we'll need `SameSite=None; Secure` and a hard look at CSRF. Out of scope today.

## Revision triggers

- We move to multiple API instances with conflicting state needs beyond what cookie-signing covers.
- We start needing to embed richer per-request claims (roles, feature flags) and don't want the DB lookup.
- A real security review says otherwise.
