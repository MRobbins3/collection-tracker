# Review notes: user-categories

Decisions made without the owner, for review. Covers user-built categories: the model and visibility rule, the field library and builder, the review lifecycle, and the web flow (ADR 0013, Proposed; builds on ADR 0009, 0010, 0012).

## Needs owner decision

- **Brand-new custom fields** (a field no approved category has, e.g. "Shoe size"). Left out: the builder only offers library fields. Why: free-form fields need decisions on allowed types (yes/no, date, number ranges), naming collisions with library fields, and whether admins vet new field names. Minimal option if wanted: let users add a text or number field with a title; it joins the library only once its category is approved.
- **Changing an approved category.** Today the creator can't edit it after approval (409). Likely shape: a `category_correction` moderation kind proposing new fields/options, like catalog corrections. Also undecided: whether option values added later by other users of a public category (not the creator) should be possible.
- **Admin edit-then-approve for categories** isn't offered (approve as built, or reject with a reason). Say if admins should be able to rename or trim fields while approving.

## Model and visibility

- **Statuses `private | pending | approved | rejected`** on `categories`, with `created_by` (NULL = system). Rejected stays usable by its creator. Alternative: a separate `user_categories` table (would split every category query in two).
- **Column default stays `'approved'`** so the reference seed and existing tests insert unchanged; the one create path sets `'private'` and a CHECK refuses an unapproved row without an owner. Alternative: default `'private'` and add `status` to the seed file.
- **Visibility rule = approved OR created_by = viewer**, one SQL fragment (`visibleTo`), applied to: `/categories` list and search, `/categories/{slug}`, `/catalog/entries` (category lookup), `POST /me/collections`, pins, the overview category list, and catalog submissions (approved only). Tests cover each with anonymous, another user, an admin, and a malformed id.
- **Collection viewers:** the owner decision says the category is visible to people allowed to see the creator's collection. Implemented by embedding the category in `GET /users/{handle}/collections/{id}` (after the privacy rule), not by opening `/categories/{slug}` to them. Why: keeps one simple rule for direct lookups and no slug-probing oracle. Alternative: run the privacy rule in `GET /categories/{slug}` for unapproved categories.
- **Admins don't see private categories in lists** (only through the queue). Alternative: an admin "all user categories" view.
- **Names:** unique among approved (case/space-insensitive) and per owner. Two users may each have a private "Sneakers"; approving the second gives 409 and the admin rejects it as a duplicate. Creating a name you can already see is a 409 with a link to that category ("reuse instead of duplicate"). Another user's private name doesn't block you and isn't revealed.
- **Slugs:** folded name + random 6 hex (`sneakers-3fa91c`), never changed, not even on approval. Why: no collisions between private categories, no existence oracle, stable links. Alternative: a clean slug on approval with a redirect from the old one.
- **Account deletion** deletes unapproved categories and the user's collections in them; approved ones stay, unattributed. Rolling back 0008 deletes unapproved user categories (with collections, items, pins) rather than making them public.

## Field library and builder

- **Library = approved categories' schema properties, de-duplicated by normalized title + type** (`string:theme`, `integer:year`). Same title with a different type is a separate field (and can't be picked together, since both would use the key `year`). Spelling and description come from the first category by name; `minimum` is kept.
- **Each source keeps its own options**, and a field picked "from Lego Sets" starts with Lego's options only. Why: a shared field's merged list grows with every approved category and would quickly pass the 50-option limit. The merged list is still used to reuse spellings ("star wars" → "Star Wars").
- **Options turn a text field into a pick list** (JSON Schema `enum`, rendered as a select; the item form had no select before, now added). No options = free text. Number fields can't have options.
- **The server composes the schema from library definitions**; the client sends only field ids and option lists. Fields already in the category being edited stay choosable even if no approved category uses them.
- **Edits can't orphan item data:** removing a field some item has a value for, or a list of options that leaves out a value an item uses (including turning free text into a list), is a 409 naming the value. Alternative: allow it and let the item form flag stale values.
- **Field order** follows jsonb key order (as seeded categories already do). Alternative: an `x-order` keyword if the owner wants user-chosen order.
- **Limits:** name 60, description 300, 20 fields, 50 options per field, 60 characters per option; the create/edit body cap is raised to 128 KB for these requests only.
- **Optional catalog identifier** ("Which detail tells two items apart?"), any chosen field.

## Lifecycle and moderation

- **New `moderation_requests.kind = 'category'`**, `category_id` = the category, no catalog entry; one pending request per category (partial unique index). Counts toward the 20-request cap and shows in My submissions ("New category", "On the public list", "Not accepted") with an "Open the category" link.
- **Pending categories are locked** (no edits until decided), so the admin approves exactly what they previewed. Alternative: allow edits while pending and show the admin the latest version.
- **Approve** sets `approved_by` / `approved_at`; approval of a name already public fails (409). **Reject** requires a reason, shown on the category page ("Reviewer’s note") and in My submissions; the creator can edit and resubmit. Merge doesn't apply to categories.
- **Admin queue** gained a "New categories" type filter; the review page shows the fields, options, identifier, and a live item form preview.

## Web

- **Entry points:** "create your own" link on Categories and "Create a category" at the bottom of the overview's "Add a category" panel; pages `/categories/new` and `/categories/{slug}/edit`. After saving you land on the category page, which shows an owner panel (status, explanation, reviewer's note, Edit, Submit for the public list).
- **Badges:** "Private", "Waiting for review", "Not approved" on category cards and in the Add-a-category panel.
- **Hydration:** SSR has no cookie, so category lists/pages render approved categories first and reload when the signed-in state resolves (`watch: [isSignedIn]`); the category page shows "Loading…" (not "not found") until auth is known.
- **Catalog UI is hidden for unapproved categories** (catalog browser, "Suggest for the catalog", "Find in catalog"), since suggestions need a public category.

## Not finished

- Everything under "Needs owner decision". Nothing else known to be incomplete.
