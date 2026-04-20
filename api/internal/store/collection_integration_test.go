//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func TestCollectionsEndToEnd(t *testing.T) {
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

	t.Run("create returns the full row including category info", func(t *testing.T) {
		c, err := colls.Create(ctx, alice.ID, "lego-sets", "My Star Wars Lego")
		require.NoError(t, err)
		require.NotEmpty(t, c.ID)
		require.Equal(t, alice.ID, c.UserID)
		require.Equal(t, "lego-sets", c.CategorySlug)
		require.Equal(t, "Lego Sets", c.CategoryName)
		require.Equal(t, "My Star Wars Lego", c.Name)
		require.Equal(t, 0, c.ItemCount)
	})

	t.Run("create with unknown category returns ErrCategoryNotFound", func(t *testing.T) {
		_, err := colls.Create(ctx, alice.ID, "not-a-real-category", "x")
		require.ErrorIs(t, err, store.ErrCategoryNotFound)
	})

	t.Run("list returns only the caller's collections", func(t *testing.T) {
		_, err := colls.Create(ctx, alice.ID, "funko-pops", "Pop Shelf")
		require.NoError(t, err)
		_, err = colls.Create(ctx, bob.ID, "coins", "Bob's Coins")
		require.NoError(t, err)

		aliceColls, err := colls.ListByUser(ctx, alice.ID)
		require.NoError(t, err)
		require.Len(t, aliceColls, 2)
		for _, c := range aliceColls {
			require.Equal(t, alice.ID, c.UserID)
		}

		bobColls, err := colls.ListByUser(ctx, bob.ID)
		require.NoError(t, err)
		require.Len(t, bobColls, 1)
		require.Equal(t, "Bob's Coins", bobColls[0].Name)
	})

	t.Run("get for the wrong user returns ErrNotFound (not 403)", func(t *testing.T) {
		c, err := colls.Create(ctx, alice.ID, "books", "Reading Pile")
		require.NoError(t, err)

		_, err = colls.GetByIDForUser(ctx, c.ID, bob.ID)
		require.ErrorIs(t, err, store.ErrNotFound, "cross-user read should hide existence")

		// same id, correct user, still works
		got, err := colls.GetByIDForUser(ctx, c.ID, alice.ID)
		require.NoError(t, err)
		require.Equal(t, c.ID, got.ID)
	})

	t.Run("rename updates only the caller's row", func(t *testing.T) {
		c, err := colls.Create(ctx, alice.ID, "stamps", "Old name")
		require.NoError(t, err)

		_, err = colls.Rename(ctx, c.ID, bob.ID, "hijacked")
		require.ErrorIs(t, err, store.ErrNotFound, "cross-user rename should fail quietly")

		updated, err := colls.Rename(ctx, c.ID, alice.ID, "New name")
		require.NoError(t, err)
		require.Equal(t, "New name", updated.Name)
		require.True(t, updated.UpdatedAt.After(c.UpdatedAt) ||
			updated.UpdatedAt.Equal(c.UpdatedAt), "updated_at should advance")
	})

	t.Run("delete removes only the caller's row", func(t *testing.T) {
		c, err := colls.Create(ctx, alice.ID, "plants", "Green")
		require.NoError(t, err)

		err = colls.Delete(ctx, c.ID, bob.ID)
		require.ErrorIs(t, err, store.ErrNotFound)

		err = colls.Delete(ctx, c.ID, alice.ID)
		require.NoError(t, err)

		_, err = colls.GetByIDForUser(ctx, c.ID, alice.ID)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("item count reflects underlying items table", func(t *testing.T) {
		c, err := colls.Create(ctx, alice.ID, "vinyl-records", "Jazz")
		require.NoError(t, err)

		// poke the items table directly; handlers for items land in M8.
		_, err = pool.Exec(ctx,
			`INSERT INTO items (collection_id, name, quantity) VALUES ($1, 'Kind of Blue', 1), ($1, 'A Love Supreme', 1)`,
			c.ID)
		require.NoError(t, err)

		got, err := colls.GetByIDForUser(ctx, c.ID, alice.ID)
		require.NoError(t, err)
		require.Equal(t, 2, got.ItemCount)
	})
}
