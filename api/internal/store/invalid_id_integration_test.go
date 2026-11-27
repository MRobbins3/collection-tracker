//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// Ids come straight from URL paths (some reachable anonymously), so a
// malformed one must be "not found", never a Postgres 22P02 → 500.
func TestMalformedIDsAreNotFound(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, err := store.NewUsers(pool).UpsertByGoogleSub(ctx, "g-1", "a@example.com", "Alice")
	require.NoError(t, err)
	colls := store.NewCollections(pool)
	items := store.NewItems(pool)
	c, err := colls.Create(ctx, u.ID, "coins", "Coins")
	require.NoError(t, err)
	it, err := items.Create(ctx, c.ID, u.ID, store.ItemInput{Name: "Penny", Quantity: 1})
	require.NoError(t, err)

	for _, bad := range []string{"not-a-uuid", "col-1", "", "'; DROP TABLE users; --"} {
		_, err := colls.GetByIDForUser(ctx, bad, u.ID)
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		_, err = colls.Rename(ctx, bad, u.ID, "x")
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		require.ErrorIs(t, colls.Delete(ctx, bad, u.ID), store.ErrNotFound, bad)

		_, err = items.ListInCollectionForUser(ctx, bad, u.ID)
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		_, err = items.Create(ctx, bad, u.ID, store.ItemInput{Name: "x", Quantity: 1})
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		_, err = items.GetInCollectionForUser(ctx, bad, c.ID, u.ID)
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		_, err = items.GetInCollectionForUser(ctx, it.ID, bad, u.ID)
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		_, err = items.Update(ctx, bad, c.ID, u.ID, store.ItemInput{Name: "x", Quantity: 1})
		require.ErrorIs(t, err, store.ErrNotFound, bad)
		require.ErrorIs(t, items.Delete(ctx, bad, c.ID, u.ID), store.ErrNotFound, bad)
	}
}
