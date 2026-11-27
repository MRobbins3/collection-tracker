//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// insertCatalogEntry writes a catalog entry straight to the table (there is
// no store write path until moderation lands). identifier mirrors the
// category's identifier attribute, as the seed does.
func insertCatalogEntry(t *testing.T, pool *pgxpool.Pool, slug, name, status string, attrs map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(attrs)
	require.NoError(t, err)
	var id string
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO catalog_entries (category_id, name, identifier, attributes, source, status)
		SELECT id, $2, $3::jsonb ->> catalog_identifier, $3::jsonb, 'seed', $4
		FROM categories WHERE slug = $1
		RETURNING id`, slug, name, string(raw), status).Scan(&id))
	return id
}

func entryNames(entries []store.CatalogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func TestCatalogBrowse(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	falcon := insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "approved",
		map[string]any{"set_number": "75192", "theme": "Star Wars"})
	insertCatalogEntry(t, pool, "lego-sets", "AT-AT", "approved", map[string]any{"set_number": "75313"})
	insertCatalogEntry(t, pool, "lego-sets", "tree house", "approved", map[string]any{"set_number": "21318"})
	insertCatalogEntry(t, pool, "lego-sets", "Galaxy Explorer", "approved", map[string]any{"set_number": "10497"})
	pending := insertCatalogEntry(t, pool, "lego-sets", "Pending Set", "pending", map[string]any{"set_number": "1"})
	insertCatalogEntry(t, pool, "lego-sets", "Rejected Set", "rejected", map[string]any{"set_number": "2"})
	insertCatalogEntry(t, pool, "vinyl-records", "Abbey Road", "approved", map[string]any{"artist": "The Beatles"})

	cat := store.NewCatalog(pool)

	t.Run("lists only approved entries in the category, by name ignoring case", func(t *testing.T) {
		page, err := cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{"AT-AT", "Galaxy Explorer", "Millennium Falcon", "tree house"}, entryNames(page.Entries))
		require.Equal(t, 4, page.Total)
		require.False(t, page.HasMore)
		require.Equal(t, "75313", *page.Entries[0].Identifier)
		require.Equal(t, "lego-sets", page.Entries[0].CategorySlug)
	})

	t.Run("unknown category is not found", func(t *testing.T) {
		_, err := cat.List(ctx, store.CatalogQuery{CategorySlug: "nope", Limit: 50})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("search matches name or identifier, best match first", func(t *testing.T) {
		page, err := cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "falcon", Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{"Millennium Falcon"}, entryNames(page.Entries))

		page, err = cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "7531", Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{"AT-AT"}, entryNames(page.Entries), "partial set number")

		page, err = cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "  TREE   house ", Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{"tree house"}, entryNames(page.Entries), "query is trimmed, collapsed, case-insensitive")

		page, err = cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "pending", Limit: 50})
		require.NoError(t, err)
		require.Empty(t, page.Entries, "pending entries never match")

		page, err = cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "%", Limit: 50})
		require.NoError(t, err)
		require.Empty(t, page.Entries, "LIKE wildcards are literal")
	})

	t.Run("get returns approved entries only", func(t *testing.T) {
		e, err := cat.Get(ctx, falcon)
		require.NoError(t, err)
		require.Equal(t, "Millennium Falcon", e.Name)
		require.Equal(t, "Lego Sets", e.CategoryName)
		require.JSONEq(t, `{"set_number":"75192","theme":"Star Wars"}`, string(e.Attributes))

		_, err = cat.Get(ctx, pending)
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = cat.Get(ctx, "not-a-uuid")
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}

func TestCatalogPagingIsStableWhenNamesTie(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx := context.Background()

	// Same name, different set numbers: the sort keys tie on name, so the
	// identifier and then the id break the tie.
	for i := 0; i < 7; i++ {
		insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "approved",
			map[string]any{"set_number": fmt.Sprintf("%d", 9-i)})
	}
	insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "pending",
		map[string]any{"set_number": "1"})
	cat := store.NewCatalog(pool)

	var seen []string
	for offset := 0; ; offset += 3 {
		page, err := cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Limit: 3, Offset: offset})
		require.NoError(t, err)
		require.Equal(t, 7, page.Total)
		for _, e := range page.Entries {
			seen = append(seen, *e.Identifier)
		}
		if !page.HasMore {
			break
		}
	}
	require.Equal(t, []string{"3", "4", "5", "6", "7", "8", "9"}, seen, "every entry exactly once, in order")
}

func TestCatalogLinkingAndSuggestions(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)
	colls := store.NewCollections(pool)
	aliceLego, err := colls.Create(ctx, alice.ID, "lego-sets", "Alice's Lego")
	require.NoError(t, err)
	aliceVinyl, err := colls.Create(ctx, alice.ID, "vinyl-records", "Alice's Records")
	require.NoError(t, err)
	bobLego, err := colls.Create(ctx, bob.ID, "lego-sets", "Bob's Lego")
	require.NoError(t, err)

	falcon := insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "approved",
		map[string]any{"set_number": "75192"})
	falcon2 := insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "approved",
		map[string]any{"set_number": "10179"})
	atat := insertCatalogEntry(t, pool, "lego-sets", "AT-AT", "approved", map[string]any{"set_number": "75313"})
	pending := insertCatalogEntry(t, pool, "lego-sets", "Pending Falcon", "pending", map[string]any{"set_number": "75192"})
	abbey := insertCatalogEntry(t, pool, "vinyl-records", "Abbey Road", "approved", map[string]any{"artist": "The Beatles"})

	items := store.NewItems(pool)
	cat := store.NewCatalog(pool)

	t.Run("create linked to an approved entry in the same category", func(t *testing.T) {
		it, err := items.Create(ctx, aliceLego.ID, alice.ID, store.ItemInput{
			Name: "Millennium Falcon", Quantity: 1, CatalogEntryID: &falcon,
		})
		require.NoError(t, err)
		require.NotNil(t, it.CatalogEntryID)
		require.Equal(t, falcon, *it.CatalogEntryID)
	})

	t.Run("create refuses pending, other-category, and unknown entries", func(t *testing.T) {
		for _, id := range []string{pending, abbey, "00000000-0000-4000-8000-000000000000", "junk"} {
			_, err := items.Create(ctx, aliceLego.ID, alice.ID, store.ItemInput{
				Name: "x", Quantity: 1, CatalogEntryID: &id,
			})
			require.ErrorIs(t, err, store.ErrCatalogEntryUnavailable, id)
		}
	})

	t.Run("update keeps the link", func(t *testing.T) {
		it, err := items.Create(ctx, aliceLego.ID, alice.ID, store.ItemInput{
			Name: "AT-AT", Quantity: 1, CatalogEntryID: &atat,
		})
		require.NoError(t, err)
		updated, err := items.Update(ctx, it.ID, aliceLego.ID, alice.ID, store.ItemInput{Name: "My AT-AT", Quantity: 2})
		require.NoError(t, err)
		require.NotNil(t, updated.CatalogEntryID)
		require.Equal(t, atat, *updated.CatalogEntryID)
	})

	t.Run("link and unlink a free-form item", func(t *testing.T) {
		it, err := items.Create(ctx, aliceLego.ID, alice.ID, store.ItemInput{Name: "Falcon", Quantity: 1})
		require.NoError(t, err)
		require.Nil(t, it.CatalogEntryID)

		linked, err := items.SetCatalogEntry(ctx, it.ID, aliceLego.ID, alice.ID, &falcon2)
		require.NoError(t, err)
		require.Equal(t, falcon2, *linked.CatalogEntryID)
		require.Equal(t, "Falcon", linked.Name, "linking never rewrites the owner's name")

		_, err = items.SetCatalogEntry(ctx, it.ID, aliceLego.ID, alice.ID, &pending)
		require.ErrorIs(t, err, store.ErrCatalogEntryUnavailable)
		_, err = items.SetCatalogEntry(ctx, it.ID, aliceLego.ID, alice.ID, &abbey)
		require.ErrorIs(t, err, store.ErrCatalogEntryUnavailable, "entry from another category")

		unlinked, err := items.SetCatalogEntry(ctx, it.ID, aliceLego.ID, alice.ID, nil)
		require.NoError(t, err)
		require.Nil(t, unlinked.CatalogEntryID)
	})

	t.Run("linking someone else's item is not found", func(t *testing.T) {
		bobs, err := items.Create(ctx, bobLego.ID, bob.ID, store.ItemInput{Name: "Bob's Falcon", Quantity: 1})
		require.NoError(t, err)
		_, err = items.SetCatalogEntry(ctx, bobs.ID, bobLego.ID, alice.ID, &falcon)
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = items.SetCatalogEntry(ctx, bobs.ID, aliceLego.ID, alice.ID, &falcon)
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = cat.Suggest(ctx, bobs.ID, bobLego.ID, alice.ID, 5)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("suggestions rank identifier+name, then identifier, then name", func(t *testing.T) {
		it, err := items.Create(ctx, aliceLego.ID, alice.ID, store.ItemInput{
			Name: "  millennium FALCON ", Quantity: 1,
			Attributes: json.RawMessage(`{"set_number": " 75192 "}`),
		})
		require.NoError(t, err)
		got, err := cat.Suggest(ctx, it.ID, aliceLego.ID, alice.ID, 5)
		require.NoError(t, err)
		require.Len(t, got, 2, "pending and unrelated entries are never suggested")
		require.Equal(t, falcon, got[0].ID)
		require.Equal(t, store.MatchNameAndIdentifier, got[0].Match)
		require.Equal(t, falcon2, got[1].ID)
		require.Equal(t, store.MatchName, got[1].Match)

		bySetNumber, err := items.Create(ctx, aliceLego.ID, alice.ID, store.ItemInput{
			Name: "Imperial walker", Quantity: 1, Attributes: json.RawMessage(`{"set_number": "75313"}`),
		})
		require.NoError(t, err)
		got, err = cat.Suggest(ctx, bySetNumber.ID, aliceLego.ID, alice.ID, 5)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, atat, got[0].ID)
		require.Equal(t, store.MatchIdentifier, got[0].Match)

		similar, err := items.Create(ctx, aliceVinyl.ID, alice.ID, store.ItemInput{Name: "Abbey Rd", Quantity: 1})
		require.NoError(t, err)
		got, err = cat.Suggest(ctx, similar.ID, aliceVinyl.ID, alice.ID, 5)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, abbey, got[0].ID)
		require.Equal(t, store.MatchSimilarName, got[0].Match)

		nothing, err := items.Create(ctx, aliceVinyl.ID, alice.ID, store.ItemInput{Name: "Zzz Unknown", Quantity: 1})
		require.NoError(t, err)
		got, err = cat.Suggest(ctx, nothing.ID, aliceVinyl.ID, alice.ID, 5)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("suggestions match large integer identifiers exactly", func(t *testing.T) {
		funko := insertCatalogEntry(t, pool, "funko-pops", "Big Number", "approved", map[string]any{"number": 1234567})
		aliceFunko, err := colls.Create(ctx, alice.ID, "funko-pops", "Alice's Pops")
		require.NoError(t, err)
		it, err := items.Create(ctx, aliceFunko.ID, alice.ID, store.ItemInput{
			Name: "Mystery figure", Quantity: 1, Attributes: json.RawMessage(`{"number": 1234567}`),
		})
		require.NoError(t, err)
		got, err := cat.Suggest(ctx, it.ID, aliceFunko.ID, alice.ID, 5)
		require.NoError(t, err)
		require.Len(t, got, 1, "1234567 must not be compared as 1.234567e+06")
		require.Equal(t, funko, got[0].ID)
		require.Equal(t, store.MatchIdentifier, got[0].Match)
	})

	t.Run("ownership lists the caller's linked items in a category only", func(t *testing.T) {
		bobs, err := items.Create(ctx, bobLego.ID, bob.ID, store.ItemInput{Name: "x", Quantity: 1, CatalogEntryID: &falcon2})
		require.NoError(t, err)
		require.NotNil(t, bobs.CatalogEntryID)

		owned, err := cat.Owned(ctx, alice.ID, "lego-sets")
		require.NoError(t, err)
		entries := map[string]bool{}
		for _, o := range owned {
			entries[o.CatalogEntryID] = true
			require.Equal(t, aliceLego.ID, o.CollectionID)
			require.Equal(t, "Alice's Lego", o.CollectionName)
		}
		require.Equal(t, map[string]bool{falcon: true, atat: true}, entries,
			"falcon2 is only Bob's (Alice unlinked hers)")

		owned, err = cat.Owned(ctx, alice.ID, "vinyl-records")
		require.NoError(t, err)
		require.Empty(t, owned)
	})
}
