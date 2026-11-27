# stack-polish — review notes

Small gaps found by a cross-stack audit of #23–#32, fixed on one branch stacked on `collection-compare`. Decisions the owner may want to revisit:

## Normalization (owner rule: normalize at write time)

- **Items and collections now go through `textnorm.Clean`** (trim + collapse whitespace, case kept): item name, condition, every top-level string attribute, collection name on create and rename. Done in both the handler (before length and schema checks, so a padded enum value like " Star  Wars " still matches its option) and the store (defense in depth). Alternative: handler only. The store copy is cheap and keeps seeds/tests/other callers honest.
- **String attributes reuse `store.CleanAttributes`** (was the private `cleanAttributes` used for catalog entries). It drops attributes that end up blank. Numbers keep their exact JSON text.
- **No case-insensitive reuse for item names or enum values.** Item and collection names are personal labels, not shared pick lists, so the "reuse the existing value" part of the rule doesn't apply; compare already matches on `textnorm.Key`. Enum values aren't case-folded to their option (the item form shows them as a dropdown, so only API callers could send another case). Alternative: canonicalize an enum value to the option that matches it case-insensitively.
- **Title Case is a suggestion button** on the item form name and "Start a new collection", same rule and wording style as the category builder and catalog suggestions (only for all-lowercase input; capitals are treated as deliberate). Not added to the collection rename box on the collection page, to keep that inline control small. Alternative: add it there too.
- **Existing rows aren't rewritten.** No migration cleans names stored before this change; everything that matches already compares on the normalized key. Alternative: a one-off data migration if stored spacing ever matters for display.

## Compare: ambiguous names

- When a free-form name matches several approved entries, the item's value for the category's `catalog_identifier` (normalized like `identifier_key`) picks the entry; otherwise name-only. A name that matches exactly one entry still matches it even if the item's set number disagrees (unchanged behavior). Alternative: require the identifier to agree whenever the item has one. Stricter, but would split matches people expect today.
- **A missing set number doesn't split a match.** A free-form item with a shared name and no usable set number, owned by only one person, joins the other person's catalog row of that name when it's the only one they have (whether theirs was set-number-matched or linked) and the first person isn't already on it. If the other person has several (both Batmobiles), it can't say which and stays by name. Alternative: always keep it by name (honest but splits "Batmobile" vs "Batmobile 76139", which matched before this branch), or attach it to every candidate (double-counts).

## Migration 0008 Down

- Edited in place (stack unmerged): Down deletes catalog entries in unapproved categories before the categories. Moderation requests on those entries cascade; links from items become NULL (those items are deleted with their collections anyway).

## Dev seed

- `0001_dev_users.sql` no longer refreshes item `created_at` on re-run; every dev sample row is now `DO NOTHING`, matching the file's own rules and 0004. A dev database whose items predate the dated seed keeps its old dates until the database is dropped and reseeded.

## Search helper

- The server's `normalizeSearchQuery` is gone (`textnorm.Clean`), and so is the web alias in `utils/search.ts`; call sites use `collapseWhitespace` from `utils/text.ts` directly.

## Docs

- ADR 0001 gains a "Nuxt 3 → Nuxt 4" correction; requirements' deferred list no longer lists user-suggested categories (shipped); next-steps notes the rate limits are per instance (matters for the AWS work).

## Not done

- Nothing outstanding.
