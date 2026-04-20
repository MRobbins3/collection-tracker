//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func TestUsersUpsertAndGet(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	users := store.NewUsers(pool)

	t.Run("insert creates a new user", func(t *testing.T) {
		u, err := users.UpsertByGoogleSub(ctx, "google-sub-a", "a@example.com", "Alice")
		require.NoError(t, err)
		require.NotEmpty(t, u.ID)
		require.Equal(t, "google-sub-a", u.GoogleSub)
		require.Equal(t, "a@example.com", u.Email)
		require.Equal(t, "Alice", u.DisplayName)
		require.False(t, u.CreatedAt.IsZero())
	})

	t.Run("upsert updates email + name, keeps id and created_at", func(t *testing.T) {
		first, err := users.UpsertByGoogleSub(ctx, "google-sub-b", "b@example.com", "Bob")
		require.NoError(t, err)

		second, err := users.UpsertByGoogleSub(ctx, "google-sub-b", "bob@new.com", "Robert")
		require.NoError(t, err)

		require.Equal(t, first.ID, second.ID, "upsert should keep the same id")
		require.WithinDuration(t, first.CreatedAt, second.CreatedAt, time.Second)
		require.Equal(t, "bob@new.com", second.Email)
		require.Equal(t, "Robert", second.DisplayName)
	})

	t.Run("GetByID returns the user", func(t *testing.T) {
		inserted, err := users.UpsertByGoogleSub(ctx, "google-sub-c", "c@example.com", "Carla")
		require.NoError(t, err)

		got, err := users.GetByID(ctx, inserted.ID)
		require.NoError(t, err)
		require.Equal(t, inserted.ID, got.ID)
		require.Equal(t, "Carla", got.DisplayName)
	})

	t.Run("GetByID with unknown id returns ErrNotFound", func(t *testing.T) {
		_, err := users.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}
