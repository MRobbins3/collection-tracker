//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func TestItemsEndToEnd(t *testing.T) {
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
	aliceColl, err := colls.Create(ctx, alice.ID, "lego-sets", "Alice's Lego")
	require.NoError(t, err)
	bobColl, err := colls.Create(ctx, bob.ID, "lego-sets", "Bob's Lego")
	require.NoError(t, err)

	items := store.NewItems(pool)
	legoAttrs, _ := json.Marshal(map[string]any{"set_number": "75192", "piece_count": 7541})

	t.Run("create + get + list", func(t *testing.T) {
		created, err := items.Create(ctx, aliceColl.ID, alice.ID, store.ItemInput{
			Name: "Millennium Falcon UCS", Quantity: 1, Attributes: legoAttrs,
		})
		require.NoError(t, err)
		require.NotEmpty(t, created.ID)
		require.Equal(t, "Millennium Falcon UCS", created.Name)

		got, err := items.GetInCollectionForUser(ctx, created.ID, aliceColl.ID, alice.ID)
		require.NoError(t, err)
		require.Equal(t, created.ID, got.ID)

		listed, err := items.ListInCollectionForUser(ctx, aliceColl.ID, alice.ID)
		require.NoError(t, err)
		require.Len(t, listed, 1)
	})

	t.Run("cross-user create in someone else's collection returns ErrNotFound", func(t *testing.T) {
		_, err := items.Create(ctx, bobColl.ID, alice.ID, store.ItemInput{
			Name: "sneaky", Quantity: 1,
		})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("cross-user read/update/delete all return ErrNotFound", func(t *testing.T) {
		bobsItem, err := items.Create(ctx, bobColl.ID, bob.ID, store.ItemInput{
			Name: "Bob's thing", Quantity: 1,
		})
		require.NoError(t, err)

		_, err = items.GetInCollectionForUser(ctx, bobsItem.ID, bobColl.ID, alice.ID)
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = items.Update(ctx, bobsItem.ID, bobColl.ID, alice.ID, store.ItemInput{Name: "hijacked", Quantity: 1})
		require.ErrorIs(t, err, store.ErrNotFound)
		err = items.Delete(ctx, bobsItem.ID, bobColl.ID, alice.ID)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("update mutates and bumps updated_at", func(t *testing.T) {
		created, err := items.Create(ctx, aliceColl.ID, alice.ID, store.ItemInput{
			Name: "old", Quantity: 1, Attributes: legoAttrs,
		})
		require.NoError(t, err)

		updated, err := items.Update(ctx, created.ID, aliceColl.ID, alice.ID, store.ItemInput{
			Name: "new", Quantity: 2, Attributes: legoAttrs,
		})
		require.NoError(t, err)
		require.Equal(t, "new", updated.Name)
		require.Equal(t, 2, updated.Quantity)
	})

	t.Run("delete removes mine", func(t *testing.T) {
		created, err := items.Create(ctx, aliceColl.ID, alice.ID, store.ItemInput{
			Name: "to delete", Quantity: 1,
		})
		require.NoError(t, err)

		require.NoError(t, items.Delete(ctx, created.ID, aliceColl.ID, alice.ID))
		_, err = items.GetInCollectionForUser(ctx, created.ID, aliceColl.ID, alice.ID)
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}
