# Review notes: item-search

Decisions made without the owner, for review. Covers the un-skipped category search e2e and MVP #6 (search your own items).

## Root cause of the skipped "search narrows the list" e2e

- **It wasn't the static `useAsyncData` key.** `watch` refetches under the same key just fine. Playwright's `fill()` typed into the server-rendered input before Vue hydrated: the input event fired before the `v-model` listener existed, then hydration reset the DOM value to the model's empty string, so the query never changed and no request was sent. Reproduced 10/10 locally on both projects once un-skipped; a probe showed the box empty again 3 s after `fill()`, and a second `fill()` after hydration sent `/categories?q=vinyl`. It "worked locally" before only when hydration happened to win the race; the cold CI dev server always lost it.
- **Fix: the box is `:disabled` until mounted (`useHydrated()`)**, so SSR markup and the first client render agree and Playwright waits for the box to be enabled. No sleeps. A negative control (removing `:disabled`) fails again. Real users on a slow load had the same bug (typed text silently vanished). Alternative: keep it enabled and copy the DOM value into the model on mount, which fights Vue's own v-model mount hook. Trade-off: without JS the box stays disabled, but search needs JS anyway.
- **Also debounced the category search (250 ms) and normalized its query**, using the same helpers as item search.

## Item search decisions

- **Inline on My collections, not a separate `/my/search` page.** Why: on a phone, the search box sits where you already are, with no extra navigation; results replace the collections list while the box has text and the list returns when it's cleared. The query is kept in `?q=` (with `router.replace`) so Back from a result restores the results and a reload keeps them. The box shows only when you have at least one collection (or a `?q=` is present). Alternative: a dedicated page reached from a header icon, which suits a global search that spans more than your items.
- **Results link to the item's collection page**, not to the item itself. There's no item detail page, and scrolling to or highlighting the item in the collection isn't done (the collection's items load client-side, so a `#item-…` anchor wouldn't land reliably). Possible follow-up.
- **Matching:** name contains the query (ILIKE, case-insensitive), or the name is a close fuzzy match (`pg_trgm` word similarity `<%`, default threshold 0.6, so "falcom" finds Millennium Falcon), or any attribute value contains the query (so "beatles" finds Abbey Road, "75192" finds the set). Condition and quantity aren't searched. LIKE wildcards in the query are escaped, so `%` and `_` are literal.
- **Ranking:** name starts with the query, then name contains it, then fuzzy name match, then attribute-only matches; ties by trigram similarity of the whole name, then name.
- **The attribute line appears only on rows that didn't match on name** (neither substring nor fuzzy), so a fuzzy name hit never shows an unrelated attribute. A row whose `attributes` isn't a JSON object (the API never writes one) is treated as having no attributes instead of failing the search.
- **The live region announces only the settled count** ("3 items found", "No items found"); "Searching…" is visual only, so screen readers aren't told on every debounced keystroke.
- **`/my` follows `?q=` changes from outside** (e.g. the header's My collections link to plain `/my` clears the box), since Nuxt reuses the page component across query changes.
- **Attribute matches show the attribute's human title from the category schema** ("Artist: The Beatles"), computed in SQL; a key with no title falls back to a humanized key ("Mint Mark"), never the raw key.
- **Index use: it's a per-user scan, not a trigram index lookup.** EXPLAIN shows Postgres finds the caller's collections via `collections_user_idx`, walks their items via `items_collection_idx`, and filters each row. Because the WHERE ORs the name match with the attribute match, the `items.name` trigram GIN index isn't used (pg_trgm still supplies the fuzzy `<%` operator). That's cheap at personal scale (hundreds to low thousands of items). If a user ever has tens of thousands, revisit (e.g. a generated `tsvector` over name + attribute values, or a UNION so the name branch can use the index).
- **Cap, not pagination:** at most 50 results, plus `has_more`; the UI then says "Showing the first 50 matches. Try a more specific search." Alternative: cursor pagination, which seemed unneeded for searching your own items.
- **Blank query → `200 {"items": [], "has_more": false}`**, without querying, rather than 400: a cleared search box is a normal state and shouldn't put an error in the console. Queries over 200 characters (the item-name limit, counted in characters) get 400.
- **Query normalization** (trim + collapse whitespace) happens in both layers: `normalizeSearchQuery` in `web/app/utils/search.ts` and `normalizeSearchQuery` in the search handler. Case is left alone (matching is case-insensitive). A shared Go normalization package didn't exist on this branch; if a sibling PR adds one, the handler's helper should switch to it.
- **No ADR.** Postgres trigram search for your own items extends the existing approach (category search, ADR-less) rather than choosing a new technology. "Postgres full-text vs. a search engine" stays in the Known Unknowns list.
- **The search result endpoint returns its own shape** (`ItemSearchResult` with `collection_name`, `category_name`, `matched_field`) instead of the full `Item` with attributes, to keep responses small.

## Not done

- Nothing in scope is unfinished. Highlighting the matched text in results and jumping to the item within its collection are possible polish follow-ups.
