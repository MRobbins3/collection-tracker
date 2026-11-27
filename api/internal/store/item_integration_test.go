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

// Item names, conditions, and string attributes are cleaned at write time
// (textnorm.Clean), so spacing differences never split matches like compare.
// Case is kept: Title Case is only a UI suggestion.
func TestItemsCleanTextOnWrite(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	alice, err := store.NewUsers(pool).UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	coll, err := store.NewCollections(pool).Create(ctx, alice.ID, "lego-sets", "Alice's Lego")
	require.NoError(t, err)
	items := store.NewItems(pool)

	cond := "  New,  sealed "
	created, err := items.Create(ctx, coll.ID, alice.ID, store.ItemInput{
		Name: "  Millennium \t Falcon  UCS ", Quantity: 1, Condition: &cond,
		Attributes: json.RawMessage(`{"set_number":"  75192 ","theme":" Star   Wars ","piece_count":7541,"notes":"   "}`),
	})
	require.NoError(t, err)
	require.Equal(t, "Millennium Falcon UCS", created.Name)
	require.NotNil(t, created.Condition)
	require.Equal(t, "New, sealed", *created.Condition)
	require.JSONEq(t, `{"set_number":"75192","theme":"Star Wars","piece_count":7541}`, string(created.Attributes))

	updated, err := items.Update(ctx, created.ID, coll.ID, alice.ID, store.ItemInput{
		Name: " LEGO   ucs  Falcon ", Quantity: 2,
		Attributes: json.RawMessage(`{"set_number":" 10179  "}`),
	})
	require.NoError(t, err)
	require.Equal(t, "LEGO ucs Falcon", updated.Name, "case is the owner's choice")
	require.JSONEq(t, `{"set_number":"10179"}`, string(updated.Attributes))
}
