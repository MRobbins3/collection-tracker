//go:build integration

package store_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/migrations"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func adminURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_ADMIN_DATABASE_URL")
	if url == "" {
		url = "postgres://collection:collection@db:5432/postgres?sslmode=disable"
	}
	return url
}

// testDBURL returns a connection string to a database named dbName on the
// same host/port as adminURL — works both in docker-compose (host=db) and
// on a GitHub Actions runner (host=localhost).
func testDBURL(t *testing.T, dbName string) string {
	t.Helper()
	return strings.Replace(adminURL(t), "/postgres?", "/"+dbName+"?", 1)
}

// provisionSeededDB creates a fresh database, runs migrations, seeds the
// category list, opens a pgx pool against it, and returns the pool + cleanup.
func provisionSeededDB(t *testing.T, name string) (*pgxpool.Pool, func()) {
	t.Helper()

	admin, err := sql.Open("pgx", adminURL(t))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, admin.PingContext(ctx))

	dbName := "ct_test_" + strings.ToLower(strings.ReplaceAll(name, "/", "_"))
	_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err, "create db %s", dbName)

	dbURL := testDBURL(t, dbName)

	require.NoError(t, migrations.Up(ctx, dbURL))

	pool, err := store.NewPool(ctx, dbURL)
	require.NoError(t, err)

	_, err = seed.Categories(ctx, pool)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		_ = admin.Close()
	}
	return pool, cleanup
}

func TestCategoriesEndToEnd(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cats := store.NewCategories(pool)

	t.Run("list returns all seeded categories in alphabetical order", func(t *testing.T) {
		got, err := cats.List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 8)

		wantOrder := []string{"Books", "Coins", "Funko Pops", "Lego Sets", "Plants", "Stamps", "Trading Cards", "Vinyl Records"}
		for i, want := range wantOrder {
			require.Equal(t, want, got[i].Name, "position %d", i)
		}
	})

	t.Run("get by slug returns the expected row", func(t *testing.T) {
		got, err := cats.GetBySlug(ctx, "lego-sets")
		require.NoError(t, err)
		require.Equal(t, "Lego Sets", got.Name)
		require.NotEmpty(t, got.AttributeSchema)
	})

	t.Run("get by unknown slug returns ErrNotFound", func(t *testing.T) {
		_, err := cats.GetBySlug(ctx, "no-such-thing")
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("search by partial name matches", func(t *testing.T) {
		got, err := cats.Search(ctx, "lego")
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(got), 1)
		require.Equal(t, "Lego Sets", got[0].Name)
	})

	t.Run("search by slug fragment matches", func(t *testing.T) {
		got, err := cats.Search(ctx, "vinyl")
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "vinyl-records", got[0].Slug)
	})

	t.Run("empty search returns full list", func(t *testing.T) {
		got, err := cats.Search(ctx, "")
		require.NoError(t, err)
		require.Len(t, got, 8)
	})
}

func TestCategoriesSeedIsIdempotent(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Seed a second time; row count must still be exactly 8.
	_, err := seed.Categories(ctx, pool)
	require.NoError(t, err)

	cats := store.NewCategories(pool)
	got, err := cats.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 8)
}
