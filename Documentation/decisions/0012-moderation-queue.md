# ADR 0012: Moderation queue, audit trail, and admin assignment

- **Status:** Accepted
- **Date:** 2026-11-27

## Context

ADR 0004 decided the shape of roles (nullable `users.role`, `auth.RequireRole("admin")` on top of `auth.Require`) and that user-submitted catalog entries land as `status = 'pending'`, `source = 'user_submitted'`. It left open how corrections to approved entries are stored, what the audit trail looks like beyond `approved_by` / `approved_at`, how concurrent admins are kept from both deciding, and how admins are appointed. A later PR adds user-built categories, which also need admin approval, so the queue must take other kinds of submissions without a second queue. ADR 0011 gave entries a natural key; submissions must reuse it to avoid duplicates.

## Decision

- **One polymorphic queue table, `moderation_requests`,** one row per thing a user asked admins to review. `kind` says what it is (`catalog_entry`, `catalog_correction`; a CHECK lists the allowed values) and each kind uses the columns it needs. A new kind (user-built categories) adds a value to the CHECK, a nullable reference column if needed, and a branch in the store's approve/reject; the queue page, "My submissions", the pending cap, and the audit columns work unchanged.
- **A new-entry suggestion is a pending `catalog_entries` row plus a request pointing at it.** ADR 0004's columns carry the entry's state, every public read keeps filtering `status = 'approved'`, and approval is a status change, not a copy. Items still link only to approved entries (ADR 0011).
- **A correction is a request with `proposed` jsonb holding only the changed fields** (`name`, `description`, `attributes`; attributes as the whole new set). The entry is untouched until approval. Proposals that change nothing are refused.
- **The natural key covers live entries only.** `catalog_entries_natural_key` becomes a unique index `WHERE status IN ('pending', 'approved')`. Pending entries reserve their name, so two users can't queue the same thing; rejected (and merged) entries stay as history without blocking a better later suggestion.
- **Duplicate detection at submit:** the cleaned name and identifier are compared with live entries. An approved match is reused (the source item is linked to it); your own pending match is returned; someone else's pending match is reported without its details. A race on the key retries the lookup once.
- **Audit trail = decision columns on the request:** `status` (pending, approved, rejected, merged), `decided_by`, `decided_at`, `decision_reason` (shown to the submitter), `merged_into_id`, and `previous` / `applied` jsonb (the entry's values before the change and what was applied, which differs from what was proposed after edit-then-approve). One decision per request, so columns suffice; a separate events table would only be needed for multi-step review. Approving an entry also sets `catalog_entries.approved_by` / `approved_at` (ADR 0004).
- **Concurrency:** every decision runs in one transaction that locks the request `FOR UPDATE`, refuses it unless pending (409 "already decided"), applies the change, and records the decision with an `UPDATE … WHERE status = 'pending'`. A second admin's transaction waits on the lock, then sees the first decision. A store test races approve against reject.
- **Merge** resolves a duplicate suggestion into an existing approved entry in the same category: the request becomes `merged`, the pending entry becomes `rejected` (never public, name freed), and the source item is linked to the existing entry.
- **Source items auto-link on approval** (or merge), if still the submitter's, still free-form, and in the same category. Chosen over letting users link items to their own pending entries, which would have made pending entries reachable through items and broken "items link only to approved entries".
- **Pending cap: 20 requests per user** (suggestions plus corrections), checked under a per-user row lock so parallel requests can't overrun it. Deciding a request frees a slot. Chosen over a time-window rate limiter: the queue's cost is pending work for admins, and a cap bounds exactly that.
- **Admin routes answer 403, not 404, to signed-in non-admins** (401 when anonymous). The 404-not-403 rule hides other users' resources; admin routes aren't anyone's resource, the web bundle already names them, and 403 makes a lost role easy to diagnose. The role is read from the users table on every request (the live-user check already loads the user), never cached in the session, so removing it takes effect immediately.
- **Admins are appointed by an operator, never self-service.** There is no API or UI to change a role. To appoint or remove one, run against the database:

  ```sql
  UPDATE users SET role = 'admin' WHERE email = 'person@example.com';
  UPDATE users SET role = NULL    WHERE email = 'person@example.com';
  ```

  Locally the dev seed makes the persona Olivia Martinez an admin.

## Consequences

- One queue, one cap, one "My submissions" list across kinds; user-built categories plug in without a schema redesign.
- Account deletion removes the user's requests (cascade) and their pending or rejected entries (explicitly, in `Users.Delete`); approved entries they submitted stay, unattributed.
- Rolling back migration 0007 fails if rejected entries share a key with live ones; delete those history rows first.
- Corrections send the whole attribute set, so two corrections approved in sequence apply in full, the later one winning per field it changed. The diff shown to the admin is always against the entry as it is now.
- Not built: notifications beyond "My submissions" (no email), unpublishing an approved entry, reviewer reputation, or editing an approved entry directly without a correction request (admins can propose and approve their own correction).
