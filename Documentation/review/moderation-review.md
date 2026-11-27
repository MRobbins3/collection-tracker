# Review notes: moderation

Decisions made without the owner, for review. Covers the admin role, user catalog suggestions, corrections, the admin moderation queue, and "My submissions" (ADR 0012, Proposed; builds on ADR 0004 "Roles and moderation" and ADR 0011).

## Needs owner decision

- None blocking. The choices below are best-practice defaults; the ones most worth a second look are the 403 for admin routes, auto-linking on approval, the 20-request cap, and the live-only natural key.

## Roles

- **`users.role text NULL CHECK (role IN ('admin'))`**, as ADR 0004 designed. Why: cheap, most rows untouched. Alternative: a roles table, if users ever need several roles.
- **Admin routes answer 403 to signed-in non-admins, 401 to anonymous.** Why: the 404-not-403 rule hides other users' resources; admin routes aren't anyone's resource and the web bundle names `/admin/moderation` anyway, and 403 makes a removed role easy to diagnose. Alternative: 404 to hide that the routes exist.
- **The role is re-read from the users table on every request** (the existing live-user check already loads the user and now puts the role on the context; `auth.RequireRole` reads it and fails closed). Why: removing a role takes effect immediately; sessions hold only the user id (ADR 0003). Alternative: role in the session cookie (stale until re-login).
- **`/me` includes `role`** (`null` for regular users) so the web can show admin navigation. Pages render inside `<AdminOnly>`, which is a courtesy; the API is the guard.
- **Admins are appointed by SQL only** (`UPDATE users SET role = 'admin' WHERE email = …`, in ADR 0012 and the README). No API, UI, or CLI. Alternative: a small `cmd/` tool or an env allowlist of admin emails, if appointing becomes frequent.
- **Dev seed: Olivia Martinez is the only admin.** The persona upsert now also sets `role`, so re-seeding restores it.

## Submissions

- **A suggestion is a pending `catalog_entries` row plus a `moderation_requests` row** pointing at it (ADR 0004's `source`/`status`/`submitted_by` columns carry it). Public reads already filter `status = 'approved'`, so pending entries can't leak. Alternative: keep suggestions in a separate table and copy on approval.
- **Duplicate detection** compares the cleaned name + identifier (ADR 0011 keys) with approved *and* pending entries. Outcomes: `already_in_catalog` (reused; the source item is linked to it on the spot), `already_submitted` (your own pending one is returned), `pending_review` (someone else's; no details shown, nothing created, your item isn't auto-linked when theirs is approved: use "Find in catalog" afterwards). Alternative for the last: attach a second requester to the same pending entry.
- **Near-duplicates (similar names) aren't flagged at submit.** The add-item search already shows close catalog matches before you suggest, and admins have Merge. Alternative: show trigram "did you mean" matches in the suggest form.
- **Auto-link on approval, not linking to your own pending entry.** The item a suggestion came from is linked when the entry is approved (or merged), if it's still yours, free-form, and in the same category. Why: keeps "items link only to approved entries" (ADR 0011), so pending entries never become reachable through items. Alternative: let the submitter link to their own pending entry right away.
- **Cap: 20 waiting requests per user** (suggestions plus corrections), enforced under a per-user row lock; 429 with a friendly message. Deciding one frees a slot. Why: bounds admin workload directly. Alternative or addition: a time-window rate limit (the repo has `keyedLimiter`).
- **Normalization at write:** names, descriptions, notes, reasons, and string attribute values are trimmed and whitespace-collapsed (`textnorm.Clean`); blank string attributes are dropped. The suggest form offers Title Case for all-lowercase names but doesn't force it. Identifiers are copied from attributes in SQL (`attributes ->> catalog_identifier`), like the seed, so numbers keep their JSON text.
- **The natural key now covers live entries only** (unique index `WHERE status IN ('pending', 'approved')`, migration 0007). Why: a rejected suggestion shouldn't block a better one with the same name forever. The dev seed's upsert changed from `ON CONFLICT ON CONSTRAINT` to the matching index predicate. Rolling 0007 back fails if rejected rows share a key with live ones.
- **Add-item flow:** "Can't find it? Suggest it for the catalog" appears when a search finds nothing; it opens manual entry with "Also suggest it for the catalog" ticked (the box is available, unticked, on plain manual entry). The page creates the item, then sends the suggestion with it. If the suggestion fails, the item is still added and a message says so.
- **Account deletion** removes the user's requests (cascade) and their pending or rejected entries (explicitly in `Users.Delete`); approved entries they submitted stay, unattributed. `TestDeleteUserCascades` changed to insert an approved entry, since pending ones are now deleted.

## Corrections

- **Stored as a request with `proposed` = only the changed fields**; attributes are sent as the whole new set and compared as JSON values (key order and number formatting don't count as changes). A proposal that changes nothing is refused (400).
- **Only approved entries can be corrected** (404 otherwise). There's no per-entry limit on concurrent corrections; each is reviewed against the entry as it is at review time, and approving applies only fields that still differ.
- **No "unpublish an approved entry" action.** So the global-catalog review's open question (does rejecting an entry unlink items?) doesn't arise: only pending entries can be rejected, and items never link to pending entries.

## Queue and decisions

- **One polymorphic `moderation_requests` table with `kind`** (`catalog_entry`, `catalog_correction`), so user-built categories add a kind instead of a second queue. Alternative: per-type tables with a UNION view.
- **Audit trail as columns on the request** (`decided_by`, `decided_at`, `status`, `decision_reason`, `merged_into_id`, `previous`, `applied`), plus `approved_by`/`approved_at` on the entry. Why: one decision per request. Alternative: an append-only events table, if multi-step review or reopening is ever needed.
- **Concurrency:** each decision locks the request `FOR UPDATE`, checks it's pending, and records with `UPDATE … WHERE status = 'pending'`; the loser gets 409 "already decided", and the review page offers Reload. Tested by racing approve against reject five times.
- **Edit-then-approve** sends the reviewer's full version (name, description, attributes); only differences are applied and recorded (`previous` = submitted/current values, `applied` = what changed). If the edit clashes with another live entry's key, approval is refused (409) and the request stays pending.
- **Merge** is for new-entry suggestions only (400 for corrections) and targets an approved entry in the same category; the suggestion's entry becomes `rejected` (never public, name freed), the request `merged`, and the submitter's item is linked to the target. The reason is optional for merges (the submitter sees "It's in the catalog as …"), required for rejections.
- **Queue order: oldest first, `id` as tie-breaker,** 25 per page with "Show more"; filters by kind and category. The web drops responses for a stale filter (generation counter). Tested with tied `created_at`.
- **Admins can review their own suggestions.** With one or two admins, forbidding it would block them. Alternative: require a different admin.

## Web

- **"My submissions"** (`/my/submissions`, account menu) lists your suggestions and corrections with status labels in words (Waiting for review, Added to the catalog, Correction applied, Already in the catalog, Not accepted), the reviewer's note, and a link to the catalog entry once there is one. Corrections show which kinds of fields change ("Name, Details") rather than per-attribute labels, since the list doesn't load each category's schema.
- **"Suggest a correction"** is on catalog entry pages for signed-in users (rendered client-side, since auth loads after hydration).
- **Admin pages:** `/admin/moderation` (queue) and `/admin/moderation/{id}` (detail with a before/after diff for corrections, labelled from the category schema; Approve, Edit then approve, Merge into existing, Reject with reason).

## Tests and other changes

- **Fixed a flaky e2e spec** (`profiles.spec.ts`, account deletion): disposable users get the lowest free `disposable_tester<N>` handle, so a parallel spec's new user could take the deleted user's handle and its profile would show. The spec now gives its user a unique handle first. The moderation specs create more disposable users in parallel, which made the flake frequent. Alternative: random disposable handles in `auth.Dev.Disposable`.
- **E2E uses the Books category with unique names per run**, because approved entries persist in a local database and the global-catalog spec counts Lego entries.

## Not done

- No email or push notifications ("My submissions" only, as scoped).
- No admin UI to appoint admins, edit approved entries directly, or browse decided requests (decided requests are viewable by URL).
