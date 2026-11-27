# Review notes: global-catalog

Decisions made without the owner, for review. Covers catalog seeding, browsing, add-from-catalog, the add-item autocomplete, and linking free-form items (ADR 0011, Proposed).

## Needs owner decision

- **Real catalog dataset pending the owner.** Only a 20-entry sample ships (8 Lego sets from Brickset, 6 albums from Wikipedia, 6 US circulating coin designs from the US Mint), cited in `api/internal/seed/sql/dev/0002_sample_catalog.sql`. Production has an empty catalog until the real dataset lands; the category page then shows "No … in the catalog yet".

## Data model

- **Sample lives in the dev seed set, not reference.** Why: it's placeholder data the owner plans to replace, so it shouldn't reach a real database. ADR 0007 had said "reference, later the sample catalog"; `Documentation/seed-data.md` now says dev, and ADR 0007 is left as a historical record. Alternative: reference set, so production has some catalog from day one.
- **Natural key = (category, normalized name, normalized identifier)**, with the identifying attribute chosen per category in `categories.catalog_identifier` (Lego set number, Funko number, stamp catalog ID, trading-card set name, book ISBN, record artist; none for coins and plants). Why: names alone collide (several "Millennium Falcon" sets), and a per-category attribute keeps the rule data-driven. Alternatives: a free-form `natural_key` column per entry, or name-only uniqueness.
- **Vinyl uses artist and trading cards use set name as the identifier.** They disambiguate rather than identify uniquely. Revisit with the real dataset (e.g. Discogs release id, card number + set).
- **Coin entries are designs (Lincoln Cent, Kennedy Half Dollar), not year/mint-mark issues.** Year and mint mark stay on the user's item. A real coin catalog may want one entry per issue.
- **Seed entries are `approved` with a fixed `approved_at` and no `approved_by`.** Re-running the seed re-approves a sample entry an admin later rejected; fine for dev data.
- **No automatic links for the dev personas' existing items.** Liam's Lego and vinyl items and Noah's coins stay free-form so "Find in catalog" has something to demo.

## API

- **`GET /catalog/entries` is paginated (24 per page, max 100, offset ≤ 10,000) with `total` and `has_more`,** sorted by normalized name, then identifier, then id (stable for ties). A search ranks exact, then prefix, then substring matches on name or identifier. It now returns 404 for an unknown category (it used to 500).
- **"In your collection" comes from a separate signed-in `GET /me/catalog-ownership?category_slug=`**, not a flag on the public list, so the public list stays cacheable and SSR-safe (SSR fetches carry no session).
- **Linking uses `PUT`/`DELETE …/items/{id}/catalog-entry`** (PUT was added to the CORS allowed methods; the overview PR had avoided PUT for that reason). Alternative: a `catalog_entry_id` field on `PATCH`, rejected because existing edit calls would silently unlink.
- **An unavailable entry (pending, other category, unknown) is a 400** on create/link with "That catalog entry isn't available for this collection." Linking someone else's item is 404.
- **Suggestions:** top 5; same name + identifier > same identifier > same name > similar name (pg_trgm similarity ≥ 0.3, its default). Matching a vinyl item by artist alone suggests that artist's other albums; acceptable as suggestions, never auto-linked.
- **Linking only sets the link.** It doesn't fill in missing attributes from the entry. Alternative: offer "also copy the catalog details" on link.

## Web

- **Catalog section sits on the category page** below the header (above "What you can track"), with search and "Show more". Entry detail is `/categories/<slug>/catalog/<id>` (the category page moved to `[slug]/index.vue`).
- **"Add to my collection" defaults to your first collection in the category**, or "A new collection" named after the category when you have none. Adding the same entry again adds another item (another copy); the panel then says "Add another copy to".
- **Autocomplete picks prefill the form and show "Linked to … in the catalog" with a "Don't link" escape;** the user can still edit everything before saving.
- **Linked items show "Linked to the catalog"** (to the entry page) and "Unlink" on your own collection page only; read-only profile views don't show catalog links yet.
- **Not done:** offline (PWA) caching of catalog pages and entries; catalog entry names on item cards (the link goes to the entry page instead).
