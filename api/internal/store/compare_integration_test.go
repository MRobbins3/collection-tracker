//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

func compareNames(rows []store.CompareRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		label := r.Name
		if r.Identifier != nil {
			label += " " + *r.Identifier
		}
		out = append(out, label)
	}
	return out
}

func rowNamed(t *testing.T, rows []store.CompareRow, name string) store.CompareRow {
	t.Helper()
	for _, r := range rows {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no row named %q in %v", name, compareNames(rows))
	return store.CompareRow{}
}

func TestCompare(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)
	carol, err := users.UpsertByGoogleSub(ctx, "carol-sub", "carol@example.com", "Carol")
	require.NoError(t, err)

	falcon := insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "approved", map[string]any{"set_number": "75192"})
	atat := insertCatalogEntry(t, pool, "lego-sets", "AT-AT", "approved", map[string]any{"set_number": "75313"})
	treeHouse := insertCatalogEntry(t, pool, "lego-sets", "Tree House", "approved", map[string]any{"set_number": "21318"})
	insertCatalogEntry(t, pool, "lego-sets", "Galaxy Explorer", "approved", map[string]any{"set_number": "10497"})
	insertCatalogEntry(t, pool, "lego-sets", "Colosseum", "approved", map[string]any{"set_number": "10276"})
	// Two approved entries share a name: a free-form "Batmobile" with no set
	// number can't say which one it is, so it's matched by name only.
	batmobileA := insertCatalogEntry(t, pool, "lego-sets", "Batmobile", "approved", map[string]any{"set_number": "76139"})
	batmobileB := insertCatalogEntry(t, pool, "lego-sets", "Batmobile", "approved", map[string]any{"set_number": "7784"})
	insertCatalogEntry(t, pool, "lego-sets", "Pending Set", "pending", map[string]any{"set_number": "1"})
	insertCatalogEntry(t, pool, "lego-sets", "Rejected Set", "rejected", map[string]any{"set_number": "2"})
	insertCatalogEntry(t, pool, "vinyl-records", "Abbey Road", "approved", map[string]any{"artist": "The Beatles"})

	colls := store.NewCollections(pool)
	items := store.NewItems(pool)
	aliceShelf, err := colls.Create(ctx, alice.ID, "lego-sets", "Shelf")
	require.NoError(t, err)
	aliceBin, err := colls.Create(ctx, alice.ID, "lego-sets", "Bin")
	require.NoError(t, err)
	bobLego, err := colls.Create(ctx, bob.ID, "lego-sets", "Bob's Lego")
	require.NoError(t, err)
	bobVinyl, err := colls.Create(ctx, bob.ID, "vinyl-records", "Bob's Records")
	require.NoError(t, err)

	day := 0
	add := func(collID, userID, name string, qty int, cond string, entry string) {
		t.Helper()
		in := store.ItemInput{Name: name, Quantity: qty}
		if cond != "" {
			in.Condition = &cond
		}
		if entry != "" {
			in.CatalogEntryID = &entry
		}
		it, err := items.Create(ctx, collID, userID, in)
		require.NoError(t, err)
		day++
		_, err = pool.Exec(ctx, `UPDATE items SET created_at = $1 WHERE id = $2`,
			time.Date(2026, 1, day, 12, 0, 0, 0, time.UTC), it.ID)
		require.NoError(t, err)
	}
	// Alice: the Falcon twice (one linked, one free-form with odd spacing)
	// in two collections, Tree House free-form, Rocket Ship free-form (no
	// catalog entry), both Batmobiles linked, and a set Bob doesn't have.
	add(aliceShelf.ID, alice.ID, "Millennium Falcon", 1, "Sealed", falcon)
	add(aliceBin.ID, alice.ID, "millennium   falcon", 2, "Built", "")
	add(aliceShelf.ID, alice.ID, "Tree House", 1, "", "")
	add(aliceShelf.ID, alice.ID, "Rocket Ship", 1, "", "")
	add(aliceShelf.ID, alice.ID, "Batmobile", 1, "", batmobileA)
	add(aliceShelf.ID, alice.ID, "Batmobile", 1, "", batmobileB)
	add(aliceShelf.ID, alice.ID, "My Moon Base", 1, "", "")
	// Bob: the Falcon free-form in capitals, Tree House linked, Rocket Ship
	// with different case and spacing, a free-form "batmobile" (ambiguous),
	// AT-AT linked, a free-form name that only matches a pending entry, and
	// a record in another category.
	add(bobLego.ID, bob.ID, "MILLENNIUM FALCON", 1, "", "")
	add(bobLego.ID, bob.ID, "Tree House", 3, "Built", treeHouse)
	add(bobLego.ID, bob.ID, " rocket   SHIP ", 1, "", "")
	add(bobLego.ID, bob.ID, "batmobile", 1, "", "")
	add(bobLego.ID, bob.ID, "AT-AT", 1, "", atat)
	add(bobLego.ID, bob.ID, "Pending Set", 1, "", "")
	add(bobVinyl.ID, bob.ID, "Abbey Road", 1, "", "")

	cmp := store.NewCompare(pool)
	run := func(viewer, other string, b store.CompareBucket, limit, offset int) store.CompareResult {
		t.Helper()
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: viewer, OtherID: other, CategorySlug: "lego-sets", Bucket: b, Limit: limit, Offset: offset,
		})
		require.NoError(t, err)
		return res
	}

	t.Run("counts every bucket", func(t *testing.T) {
		res := run(alice.ID, bob.ID, store.BucketBoth, 50, 0)
		require.Equal(t, store.CompareCounts{Both: 3, MineOnly: 3, TheirsOnly: 3, Neither: 2}, res.Counts)
		require.Equal(t, "lego-sets", res.Category.Slug)
		require.Equal(t, "Lego Sets", res.Category.Name)
		require.True(t, res.Category.Public)
		require.Equal(t, 7, res.Category.CatalogEntries, "approved entries only")
	})

	t.Run("both: catalog links first, then a free-form name that names exactly one entry, then normalized names", func(t *testing.T) {
		res := run(alice.ID, bob.ID, store.BucketBoth, 50, 0)
		require.Equal(t, []string{"Millennium Falcon 75192", "Rocket Ship", "Tree House 21318"}, compareNames(res.Items))
		require.False(t, res.HasMore)

		f := rowNamed(t, res.Items, "Millennium Falcon")
		require.NotNil(t, f.CatalogEntryID)
		require.Equal(t, falcon, *f.CatalogEntryID)
		require.Equal(t, 3, f.Mine.Quantity, "quantities add up across copies and collections")
		require.Equal(t, 2, f.Mine.Items)
		require.Equal(t, []string{"Built", "Sealed"}, f.Mine.Conditions)
		require.Equal(t, aliceShelf.ID, f.Mine.CollectionID, "the earliest copy's collection")
		require.Equal(t, 1, f.Theirs.Quantity)
		require.Equal(t, bobLego.ID, f.Theirs.CollectionID)
		require.Equal(t, "Bob's Lego", f.Theirs.CollectionName)

		r := rowNamed(t, res.Items, "Rocket Ship")
		require.Nil(t, r.CatalogEntryID)
		require.Nil(t, r.Identifier)

		th := rowNamed(t, res.Items, "Tree House")
		require.Equal(t, treeHouse, *th.CatalogEntryID)
		require.Equal(t, 3, th.Theirs.Quantity)
	})

	t.Run("mine only and theirs only", func(t *testing.T) {
		mine := run(alice.ID, bob.ID, store.BucketMineOnly, 50, 0)
		require.Equal(t, []string{"Batmobile 76139", "Batmobile 7784", "My Moon Base"}, compareNames(mine.Items))
		for _, r := range mine.Items {
			require.NotNil(t, r.Mine)
			require.Nil(t, r.Theirs)
		}

		theirs := run(alice.ID, bob.ID, store.BucketTheirsOnly, 50, 0)
		require.Equal(t, []string{"AT-AT 75313", "batmobile", "Pending Set"}, compareNames(theirs.Items),
			"an ambiguous name and a pending entry's name stay free-form")
		for _, r := range theirs.Items {
			require.Nil(t, r.Mine)
			require.NotNil(t, r.Theirs)
		}
	})

	t.Run("neither lists approved entries nobody has, with the entry to add", func(t *testing.T) {
		res := run(alice.ID, bob.ID, store.BucketNeither, 50, 0)
		require.Equal(t, []string{"Colosseum 10276", "Galaxy Explorer 10497"}, compareNames(res.Items))
		for _, r := range res.Items {
			require.Nil(t, r.Mine)
			require.Nil(t, r.Theirs)
			require.NotNil(t, r.Entry)
			require.Equal(t, "approved", r.Entry.Status)
			require.Equal(t, "lego-sets", r.Entry.CategorySlug)
		}
	})

	t.Run("swapping sides swaps mine and theirs", func(t *testing.T) {
		res := run(bob.ID, alice.ID, store.BucketMineOnly, 50, 0)
		require.Equal(t, []string{"AT-AT 75313", "batmobile", "Pending Set"}, compareNames(res.Items))
		require.Equal(t, store.CompareCounts{Both: 3, MineOnly: 3, TheirsOnly: 3, Neither: 2}, res.Counts)
	})

	t.Run("pages stably, even across tied names", func(t *testing.T) {
		for _, b := range []store.CompareBucket{store.BucketBoth, store.BucketMineOnly, store.BucketTheirsOnly, store.BucketNeither} {
			all := run(alice.ID, bob.ID, b, 50, 0)
			var paged []string
			for off := 0; ; off++ {
				p := run(alice.ID, bob.ID, b, 1, off)
				paged = append(paged, compareNames(p.Items)...)
				if !p.HasMore {
					break
				}
				require.Len(t, p.Items, 1)
			}
			require.Equal(t, compareNames(all.Items), paged, string(b))
		}
	})

	t.Run("works when the other person has nothing in the category", func(t *testing.T) {
		res := run(alice.ID, carol.ID, store.BucketMineOnly, 50, 0)
		require.Equal(t, store.CompareCounts{Both: 0, MineOnly: 6, TheirsOnly: 0, Neither: 3}, res.Counts)
		require.Len(t, res.Items, 6)
	})

	t.Run("a category with no catalog has nothing in neither", func(t *testing.T) {
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: "stamps", Bucket: store.BucketNeither, Limit: 50,
		})
		require.NoError(t, err)
		require.Equal(t, store.CompareCounts{}, res.Counts)
		require.Zero(t, res.Category.CatalogEntries)
		require.Empty(t, res.Items)
	})

	t.Run("unknown category, or comparing with yourself", func(t *testing.T) {
		_, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: "nope", Bucket: store.BucketBoth, Limit: 50,
		})
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = cmp.Compare(ctx, store.CompareQuery{
			ViewerID: alice.ID, OtherID: alice.ID, CategorySlug: "lego-sets", Bucket: store.BucketBoth, Limit: 50,
		})
		require.ErrorIs(t, err, store.ErrCompareSelf)
	})
}

// When a free-form name matches more than one approved entry, the item's
// value for the category's catalog identifier (set number for Lego) picks
// the entry; without a matching identifier it falls back to name-only.
func TestCompareDisambiguatesSharedNamesByIdentifier(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)

	batmobileA := insertCatalogEntry(t, pool, "lego-sets", "Batmobile", "approved", map[string]any{"set_number": "76139"})
	batmobileB := insertCatalogEntry(t, pool, "lego-sets", "Batmobile", "approved", map[string]any{"set_number": "7784"})
	insertCatalogEntry(t, pool, "lego-sets", "Tree House", "approved", map[string]any{"set_number": "21318"})

	colls := store.NewCollections(pool)
	items := store.NewItems(pool)
	aliceLego, err := colls.Create(ctx, alice.ID, "lego-sets", "Alice's Lego")
	require.NoError(t, err)
	bobLego, err := colls.Create(ctx, bob.ID, "lego-sets", "Bob's Lego")
	require.NoError(t, err)
	add := func(collID, userID, name, attrs, entry string) {
		t.Helper()
		in := store.ItemInput{Name: name, Quantity: 1, Attributes: json.RawMessage(attrs)}
		if entry != "" {
			in.CatalogEntryID = &entry
		}
		_, err := items.Create(ctx, collID, userID, in)
		require.NoError(t, err)
	}
	// Free-form with a known set number matches that entry, whichever side
	// linked theirs; a set number given as a JSON number still counts.
	add(aliceLego.ID, alice.ID, "Batmobile", `{"set_number":"76139"}`, "")
	add(bobLego.ID, bob.ID, "Batmobile", `{}`, batmobileA)
	add(aliceLego.ID, alice.ID, "Batmobile", `{}`, batmobileB)
	add(bobLego.ID, bob.ID, "batmobile", `{"set_number":7784}`, "")
	// An unknown set number, or none, falls back to the name.
	add(aliceLego.ID, alice.ID, "BATMOBILE", `{"set_number":"1"}`, "")
	add(bobLego.ID, bob.ID, "Batmobile", `{}`, "")

	res, err := store.NewCompare(pool).Compare(ctx, store.CompareQuery{
		ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: "lego-sets", Bucket: store.BucketBoth, Limit: 50,
	})
	require.NoError(t, err)
	require.Equal(t, store.CompareCounts{Both: 3, Neither: 1}, res.Counts)
	require.Equal(t, []string{"BATMOBILE", "Batmobile 76139", "Batmobile 7784"}, compareNames(res.Items))
	require.Equal(t, batmobileA, *res.Items[1].CatalogEntryID)
	require.Equal(t, batmobileB, *res.Items[2].CatalogEntryID)
	require.Nil(t, res.Items[0].CatalogEntryID)
}

// A free-form item with a shared name and no set number still matches the
// other person's only item of that name, even when theirs was resolved to an
// entry by its set number. If they have several, it can't say which and
// stays by name.
func TestCompareMatchesANumberlessNameToTheOtherSidesOnlyMatch(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)
	carol, err := users.UpsertByGoogleSub(ctx, "carol-sub", "carol@example.com", "Carol")
	require.NoError(t, err)

	batmobileA := insertCatalogEntry(t, pool, "lego-sets", "Batmobile", "approved", map[string]any{"set_number": "76139"})
	insertCatalogEntry(t, pool, "lego-sets", "Batmobile", "approved", map[string]any{"set_number": "7784"})

	colls := store.NewCollections(pool)
	items := store.NewItems(pool)
	add := func(userID, name, attrs string) {
		t.Helper()
		coll, err := colls.Create(ctx, userID, "lego-sets", "Lego")
		require.NoError(t, err)
		_, err = items.Create(ctx, coll.ID, userID, store.ItemInput{Name: name, Quantity: 1, Attributes: json.RawMessage(attrs)})
		require.NoError(t, err)
	}
	add(alice.ID, "Batmobile", `{"set_number":"76139"}`)
	add(bob.ID, "batmobile", `{}`)
	add(carol.ID, "Batmobile", `{"set_number":"76139"}`)
	add(carol.ID, "Batmobile", `{"set_number":"7784"}`)

	cmp := store.NewCompare(pool)
	run := func(viewer, other string, b store.CompareBucket) store.CompareResult {
		t.Helper()
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: viewer, OtherID: other, CategorySlug: "lego-sets", Bucket: b, Limit: 50,
		})
		require.NoError(t, err)
		return res
	}

	t.Run("one numbered match on the other side: both own it", func(t *testing.T) {
		for _, viewer := range []string{alice.ID, bob.ID} {
			other := bob.ID
			if viewer == bob.ID {
				other = alice.ID
			}
			res := run(viewer, other, store.BucketBoth)
			require.Equal(t, store.CompareCounts{Both: 1, Neither: 1}, res.Counts)
			require.Equal(t, []string{"Batmobile 76139"}, compareNames(res.Items))
			require.Equal(t, batmobileA, *res.Items[0].CatalogEntryID)
			require.NotNil(t, res.Items[0].Mine)
			require.NotNil(t, res.Items[0].Theirs)
		}
	})

	t.Run("several on the other side: it stays by name", func(t *testing.T) {
		res := run(bob.ID, carol.ID, store.BucketMineOnly)
		require.Equal(t, store.CompareCounts{MineOnly: 1, TheirsOnly: 2}, res.Counts)
		require.Equal(t, []string{"batmobile"}, compareNames(res.Items))
	})
}

// Unapproved categories follow ADR 0013: your own are visible, and so is
// any category the other person has a collection in (the caller has already
// applied the privacy rule, and their profile shows those collections). A
// category the other person built but has no collection in stays hidden, as
// does a third person's with no collection of theirs.
func TestCompareCategoryVisibility(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)
	carol, err := users.UpsertByGoogleSub(ctx, "carol-sub", "carol@example.com", "Carol")
	require.NoError(t, err)

	mkCategory := func(slug, name, owner string) {
		t.Helper()
		_, err := pool.Exec(ctx, `INSERT INTO categories (slug, name, status, created_by) VALUES ($1, $2, 'private', $3)`,
			slug, name, owner)
		require.NoError(t, err)
	}
	mkCategory("sneakers-aaaaaa", "Sneakers", alice.ID)
	mkCategory("keychains-bbbbbb", "Keychains", bob.ID)
	mkCategory("spoons-cccccc", "Spoons", carol.ID)
	mkCategory("magnets-dddddd", "Magnets", bob.ID)
	mkCategory("thimbles-eeeeee", "Thimbles", carol.ID)

	colls := store.NewCollections(pool)
	items := store.NewItems(pool)
	mkItem := func(userID, slug, name string) {
		t.Helper()
		// Straight to the table: Bob can't open a collection in Carol's
		// private category through the store (it was approved once, say).
		var collID string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO collections (user_id, category_id, name)
			SELECT $1, id, $3 FROM categories WHERE slug = $2 RETURNING id`, userID, slug, name+" collection").Scan(&collID))
		_, err := items.Create(ctx, collID, userID, store.ItemInput{Name: name, Quantity: 1})
		require.NoError(t, err)
	}
	mkItem(alice.ID, "sneakers-aaaaaa", "Air Max 90")
	mkItem(bob.ID, "keychains-bbbbbb", "Eiffel Tower")
	mkItem(bob.ID, "spoons-cccccc", "Niagara Falls")
	mkItem(alice.ID, "coins", "Lincoln Cent")
	mkItem(bob.ID, "coins", "lincoln cent")
	mkItem(bob.ID, "coins", "Buffalo Nickel")
	_, err = colls.Create(ctx, bob.ID, "stamps", "Empty album")
	require.NoError(t, err)

	cmp := store.NewCompare(pool)

	t.Run("the picker lists visible categories either person has items in", func(t *testing.T) {
		cats, err := cmp.Categories(ctx, alice.ID, bob.ID)
		require.NoError(t, err)
		require.Equal(t, []store.CompareCategory{
			{Slug: "coins", Name: "Coins", Public: true, MineItems: 1, TheirItems: 2},
			{Slug: "keychains-bbbbbb", Name: "Keychains", Public: false, MineItems: 0, TheirItems: 1},
			{Slug: "sneakers-aaaaaa", Name: "Sneakers", Public: false, MineItems: 1, TheirItems: 0},
			{Slug: "spoons-cccccc", Name: "Spoons", Public: false, MineItems: 0, TheirItems: 1},
		}, cats, "Bob's collection in Carol's Spoons shows on his profile; an empty collection adds nothing")
	})

	t.Run("the other person's unapproved category compares", func(t *testing.T) {
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: "keychains-bbbbbb", Bucket: store.BucketTheirsOnly, Limit: 50,
		})
		require.NoError(t, err)
		require.False(t, res.Category.Public)
		require.Equal(t, []string{"Eiffel Tower"}, compareNames(res.Items))
	})

	t.Run("a third person's category the other person collects in compares", func(t *testing.T) {
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: "spoons-cccccc", Bucket: store.BucketTheirsOnly, Limit: 50,
		})
		require.NoError(t, err)
		require.Equal(t, []string{"Niagara Falls"}, compareNames(res.Items))
	})

	t.Run("an unapproved category with no collection of theirs is not found", func(t *testing.T) {
		for _, slug := range []string{"magnets-dddddd", "thimbles-eeeeee"} {
			_, err := cmp.Compare(ctx, store.CompareQuery{
				ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: slug, Bucket: store.BucketTheirsOnly, Limit: 50,
			})
			require.ErrorIs(t, err, store.ErrNotFound, slug)
		}
	})

	t.Run("free-form names match across case", func(t *testing.T) {
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: alice.ID, OtherID: bob.ID, CategorySlug: "coins", Bucket: store.BucketBoth, Limit: 50,
		})
		require.NoError(t, err)
		require.Equal(t, store.CompareCounts{Both: 1, MineOnly: 0, TheirsOnly: 1, Neither: 0}, res.Counts)
		require.Equal(t, []string{"Lincoln Cent"}, compareNames(res.Items), "the viewer's spelling is shown")
	})
}
