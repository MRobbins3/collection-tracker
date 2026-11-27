# Review notes: collection-compare

Decisions made without the owner, for review. Owner decision followed as given: "neither owns" is the category's approved catalog entries neither person has, and items without a catalog link are matched by normalized name.

## Needs owner decision

- Nothing blocking. The matching rule below is the one most worth a look.

## Matching

- **Match order: catalog link, then a free-form name that names exactly one approved entry, then normalized name.** Liam's seeded sets are free-form; Noah's Tree House is linked. Without the middle step they wouldn't match, and the Tree House entry would also show under "Neither of you" although Liam has one. Alternative: link-or-name only (simpler, but mixed linked/free-form pairs never match).
- **Ambiguous names stay free-form.** Two approved "Batmobile" entries (different set numbers): a free-form "Batmobile" matches other free-form "Batmobile"s by name only, and both entries can still show under "Neither of you". Alternative: also match on the identifier attribute (set number) when the item has one. That's more accurate for Lego but means trusting item attributes. A reasonable follow-up.
- **Normalized name = `textnorm.Key`** (trim, collapse whitespace, lowercase), the same helper as everywhere else. No fuzzy or typo-tolerant matching: "Millenium Falcon" doesn't match "Millennium Falcon".
- **Owning one copy is enough.** Quantities and copies are summed per person for display ("3 copies · Built, Sealed"), with no "you have 3, they have 1" diff bucket. Items with quantity 0 (only possible by direct DB edit) still count as owned.
- **Pending or rejected entries never appear under "Neither of you"**, and a free-form name matching only a pending entry stays free-form.
- **Labels:** catalog-matched rows show the entry's name and identifier, whatever the owners called their copies. Free-form rows show the viewer's spelling when they have one, else the other person's. Each side links to the collection holding its earliest copy.

## Privacy and visibility

- **Same rule as the profile:** comparing goes through `resolveView`, so it's 404 unless the viewer may see the other person's collections (not just their card). Collection visibility is account-wide (ADR 0009), so "only the items they're allowed to see" means all or nothing.
- **Signed in only** (401 otherwise): there's no "mine" without a viewer. Comparing with yourself is 400 and the page says "That's you".
- **Categories:** approved, the viewer's own, and the other person's own unapproved categories (the viewer sees those through the collection view already, ADR 0013). A third person's unapproved category is 404 and is left out of the picker. This is the one widening of `visibleTo`, named `compareVisible` and noted in CLAUDE.md.
- **Unknown or hidden category is 404 "category not found"**, not 400, so slugs can't be probed. The page drops it from the URL and picks a category.

## API shape

- **One endpoint, one bucket per request:** `GET /users/{handle}/compare?category=&bucket=&limit=&offset=` returns all four counts plus one page (default 50, max 100) of the open tab. Without `category` it returns only the picker. Offset paging over a stable order (normalized name, identifier, then the unique match key; "neither" uses the catalog's order: name key, identifier key, id). Alternative: all four buckets in one response, capped. Simpler, but a large catalog makes "neither" heavy.
- **Matching runs in Go** over both people's items in the category (one query), not in SQL. Fine at collection sizes (hundreds to low thousands); "neither" is paged in SQL. If collections get huge, move matching into SQL with a stored normalized name on `items`.
- **No ADR:** this reuses the privacy, category, and catalog models without changing them.

## UI

- **Entry points:** "Compare collections" next to the Collections heading on a profile (signed-in visitor, collections visible, at least one collection); "Compare with my <category>" on someone's collection page. No entry in the header or People list. Alternative: a compare button on each friend row in People.
- **Default category:** the first (by name) that both people have items in, else the first. URL keeps `?category=` and `?tab=` (replace, not push, so Back leaves the page).
- **"Add to my collection"** reuses the catalog page's `AddToCollection` panel inline; after adding, the comparison reloads and says "Added X to Y." `AddToCollection` gained an `added` event.
- **Tabs name the other person by first name** ("Only Noah"), split on the first space of the display name.

## Dev seed

- `api/internal/seed/sql/dev/0004_compare_sample.sql` gives Noah a "Lego Shelf" (Tree House and Typewriter linked to the catalog, "galaxy explorer" free-form in lowercase, NASA Apollo Saturn V not in the sample catalog). Real sets, from Brickset. No Millennium Falcon for Noah: an e2e spec relies on Noah not having one. The catalog sample is still the dev placeholder; the real dataset is pending (next-steps #1).
