//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func TestOverview(t *testing.T) {
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
	aliceLego, err := colls.Create(ctx, alice.ID, "lego-sets", "Lego Shelf")
	require.NoError(t, err)
	aliceLego2, err := colls.Create(ctx, alice.ID, "lego-sets", "Lego Bin")
	require.NoError(t, err)
	aliceVinyl, err := colls.Create(ctx, alice.ID, "vinyl-records", "Records")
	require.NoError(t, err)
	// An empty collection: the category is in the overview but has no items.
	_, err = colls.Create(ctx, alice.ID, "stamps", "Stamp Album")
	require.NoError(t, err)
	bobCoins, err := colls.Create(ctx, bob.ID, "coins", "Bob's Coins")
	require.NoError(t, err)

	items := store.NewItems(pool)
	add := func(collID, userID, name string, createdAt time.Time) store.Item {
		t.Helper()
		it, err := items.Create(ctx, collID, userID, store.ItemInput{Name: name, Quantity: 1})
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE items SET created_at = $1 WHERE id = $2`, createdAt, it.ID)
		require.NoError(t, err)
		return it
	}
	day := func(n int) time.Time { return time.Date(2026, 1, n, 12, 0, 0, 0, time.UTC) }
	add(aliceLego.ID, alice.ID, "Tree House", day(1))
	add(aliceVinyl.ID, alice.ID, "abbey Road", day(4)) // lowercase first letter: name sort ignores case
	add(aliceLego2.ID, alice.ID, "Millennium Falcon", day(3))
	add(aliceVinyl.ID, alice.ID, "Rumours", day(2))
	add(bobCoins.ID, bob.ID, "Buffalo Nickel", day(5))

	ov := store.NewOverview(pool)

	names := func(p store.OverviewItemsPage) []string {
		out := make([]string, 0, len(p.Items))
		for _, it := range p.Items {
			out = append(out, it.Name)
		}
		return out
	}
	list := func(q store.OverviewQuery) store.OverviewItemsPage {
		t.Helper()
		if q.Limit == 0 {
			q.Limit = 50
		}
		p, err := ov.Items(ctx, alice.ID, q)
		require.NoError(t, err)
		return p
	}

	t.Run("sorts the caller's items", func(t *testing.T) {
		cases := map[store.OverviewSort][]string{
			store.SortNewest:   {"abbey Road", "Millennium Falcon", "Rumours", "Tree House"},
			store.SortOldest:   {"Tree House", "Rumours", "Millennium Falcon", "abbey Road"},
			store.SortNameAsc:  {"abbey Road", "Millennium Falcon", "Rumours", "Tree House"},
			store.SortNameDesc: {"Tree House", "Rumours", "Millennium Falcon", "abbey Road"},
			// Category name, then item name.
			store.SortCategory: {"Millennium Falcon", "Tree House", "abbey Road", "Rumours"},
		}
		for sort, want := range cases {
			t.Run(string(sort), func(t *testing.T) {
				p := list(store.OverviewQuery{Sort: sort})
				require.Equal(t, want, names(p))
				require.Equal(t, 4, p.Total)
				require.False(t, p.HasMore)
			})
		}
	})

	t.Run("items carry their collection and category labels", func(t *testing.T) {
		p := list(store.OverviewQuery{Sort: store.SortOldest})
		first := p.Items[0]
		require.Equal(t, "Tree House", first.Name)
		require.Equal(t, aliceLego.ID, first.CollectionID)
		require.Equal(t, "Lego Shelf", first.CollectionName)
		require.Equal(t, "lego-sets", first.CategorySlug)
		require.Equal(t, "Lego Sets", first.CategoryName)
		require.Equal(t, 1, first.Quantity)
		require.True(t, first.CreatedAt.Equal(day(1)))
	})

	t.Run("filters by one or more categories", func(t *testing.T) {
		p := list(store.OverviewQuery{Sort: store.SortNameAsc, CategorySlugs: []string{"vinyl-records"}})
		require.Equal(t, []string{"abbey Road", "Rumours"}, names(p))
		require.Equal(t, 2, p.Total)

		p = list(store.OverviewQuery{Sort: store.SortNameAsc, CategorySlugs: []string{"vinyl-records", "lego-sets"}})
		require.Len(t, p.Items, 4)

		p = list(store.OverviewQuery{Sort: store.SortNameAsc, CategorySlugs: []string{"coins"}})
		require.Empty(t, p.Items, "bob's coins never show up in alice's overview")
		require.Equal(t, 0, p.Total)

		p = list(store.OverviewQuery{Sort: store.SortNameAsc, CategorySlugs: []string{"no-such-category"}})
		require.Empty(t, p.Items)
	})

	t.Run("pages with limit and offset", func(t *testing.T) {
		p := list(store.OverviewQuery{Sort: store.SortNameAsc, Limit: 3})
		require.Equal(t, []string{"abbey Road", "Millennium Falcon", "Rumours"}, names(p))
		require.True(t, p.HasMore)
		require.Equal(t, 4, p.Total)

		p = list(store.OverviewQuery{Sort: store.SortNameAsc, Limit: 3, Offset: 3})
		require.Equal(t, []string{"Tree House"}, names(p))
		require.False(t, p.HasMore)
		require.Equal(t, 4, p.Total)
	})

	t.Run("never returns another user's items", func(t *testing.T) {
		p, err := ov.Items(ctx, bob.ID, store.OverviewQuery{Sort: store.SortNewest, Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{"Buffalo Nickel"}, names(p))
	})

	t.Run("a malformed user id returns nothing rather than an error", func(t *testing.T) {
		p, err := ov.Items(ctx, "not-a-uuid", store.OverviewQuery{Sort: store.SortNewest, Limit: 50})
		require.NoError(t, err)
		require.Empty(t, p.Items)
		cats, err := ov.Categories(ctx, "not-a-uuid")
		require.NoError(t, err)
		require.Empty(t, cats)
	})

	findCat := func(cats []store.OverviewCategory, slug string) *store.OverviewCategory {
		for i := range cats {
			if cats[i].Slug == slug {
				return &cats[i]
			}
		}
		return nil
	}

	t.Run("categories list the caller's categories with counts, by name", func(t *testing.T) {
		cats, err := ov.Categories(ctx, alice.ID)
		require.NoError(t, err)
		slugs := make([]string, 0, len(cats))
		for _, c := range cats {
			slugs = append(slugs, c.Slug)
		}
		require.Equal(t, []string{"lego-sets", "stamps", "vinyl-records"}, slugs)

		lego := findCat(cats, "lego-sets")
		require.Equal(t, "Lego Sets", lego.Name)
		require.Equal(t, 2, lego.ItemCount)
		require.Equal(t, 2, lego.CollectionCount)
		require.False(t, lego.Pinned)

		stamps := findCat(cats, "stamps")
		require.Equal(t, 0, stamps.ItemCount, "an empty collection makes an empty category")
		require.Equal(t, 1, stamps.CollectionCount)

		require.Nil(t, findCat(cats, "coins"), "bob's categories aren't alice's")
	})

	t.Run("pinning adds an empty category; unpinning removes it", func(t *testing.T) {
		require.NoError(t, ov.Pin(ctx, alice.ID, "plants"))
		require.NoError(t, ov.Pin(ctx, alice.ID, "plants"), "pinning twice is a no-op")

		cats, err := ov.Categories(ctx, alice.ID)
		require.NoError(t, err)
		plants := findCat(cats, "plants")
		require.NotNil(t, plants)
		require.True(t, plants.Pinned)
		require.Equal(t, 0, plants.ItemCount)
		require.Equal(t, 0, plants.CollectionCount)

		bobCats, err := ov.Categories(ctx, bob.ID)
		require.NoError(t, err)
		require.Nil(t, findCat(bobCats, "plants"), "pins are per user")

		require.NoError(t, ov.Unpin(ctx, alice.ID, "plants"))
		require.NoError(t, ov.Unpin(ctx, alice.ID, "plants"), "unpinning twice is a no-op")
		cats, err = ov.Categories(ctx, alice.ID)
		require.NoError(t, err)
		require.Nil(t, findCat(cats, "plants"))
	})

	t.Run("pinning a category you already collect marks it pinned without duplicating it", func(t *testing.T) {
		require.NoError(t, ov.Pin(ctx, alice.ID, "lego-sets"))
		cats, err := ov.Categories(ctx, alice.ID)
		require.NoError(t, err)
		require.Len(t, cats, 3)
		require.True(t, findCat(cats, "lego-sets").Pinned)
		require.NoError(t, ov.Unpin(ctx, alice.ID, "lego-sets"))
		cats, err = ov.Categories(ctx, alice.ID)
		require.NoError(t, err)
		require.NotNil(t, findCat(cats, "lego-sets"), "unpinning keeps a category you have collections in")
	})

	t.Run("pinning an unknown category returns ErrCategoryNotFound", func(t *testing.T) {
		require.ErrorIs(t, ov.Pin(ctx, alice.ID, "no-such-category"), store.ErrCategoryNotFound)
		require.NoError(t, ov.Unpin(ctx, alice.ID, "no-such-category"))
	})

	t.Run("show-empty setting defaults on and persists per user", func(t *testing.T) {
		s, err := ov.Settings(ctx, alice.ID)
		require.NoError(t, err)
		require.True(t, s.ShowEmptyCategories)

		s, err = ov.UpdateSettings(ctx, alice.ID, store.OverviewSettings{ShowEmptyCategories: false})
		require.NoError(t, err)
		require.False(t, s.ShowEmptyCategories)

		s, err = ov.Settings(ctx, alice.ID)
		require.NoError(t, err)
		require.False(t, s.ShowEmptyCategories)

		s, err = ov.Settings(ctx, bob.ID)
		require.NoError(t, err)
		require.True(t, s.ShowEmptyCategories, "one user's setting doesn't touch another's")

		_, err = ov.Settings(ctx, "not-a-uuid")
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("deleting the user deletes their pins", func(t *testing.T) {
		require.NoError(t, ov.Pin(ctx, bob.ID, "plants"))
		require.NoError(t, users.Delete(ctx, bob.ID))
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM user_pinned_categories WHERE user_id = $1`, bob.ID).Scan(&n))
		require.Zero(t, n)
	})
}
