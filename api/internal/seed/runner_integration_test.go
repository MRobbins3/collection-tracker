//go:build integration

package seed_test

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/migrations"
	"github.com/MRobbins3/collection-tracker/api/internal/seed"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

func adminURL() string {
	if url := os.Getenv("TEST_ADMIN_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://collection:collection@db:5432/postgres?sslmode=disable"
}

// provisionMigratedDB creates a disposable database (ADR 0002), runs the
// goose migrations, and returns a pool against it. No seeds are applied.
func provisionMigratedDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	admin, err := sql.Open("pgx", adminURL())
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, admin.PingContext(ctx))

	dbName := "ct_test_seed_" + strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_"))
	_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err, "create db %s", dbName)

	dbURL := strings.Replace(adminURL(), "/postgres?", "/"+dbName+"?", 1)
	require.NoError(t, migrations.Up(ctx, dbURL))

	pool, err := store.NewPool(ctx, dbURL)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		_ = admin.Close()
	})
	return pool
}

// markerFS is a tiny seed tree: one reference file and two dev files, named
// so that lexical order matters (the second dev file depends on the first).
func markerFS() fstest.MapFS {
	return fstest.MapFS{
		"reference/0001_ref.sql": {Data: []byte(`
			INSERT INTO categories (slug, name) VALUES ('ref-marker', 'Ref')
			ON CONFLICT (slug) DO NOTHING;`)},
		"dev/0001_dev.sql": {Data: []byte(`
			INSERT INTO categories (slug, name) VALUES ('dev-marker', 'Dev')
			ON CONFLICT (slug) DO NOTHING;`)},
		"dev/0002_dev_depends.sql": {Data: []byte(`
			UPDATE categories SET description = 'touched'
			WHERE slug = 'dev-marker';`)},
	}
}

func slugExists(t *testing.T, pool *pgxpool.Pool, slug string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM categories WHERE slug = $1)`, slug,
	).Scan(&exists))
	return exists
}

func TestDevSeedsOnlyApplyWhenEnabled(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	r := &seed.Runner{FS: markerFS()}

	applied, err := r.Apply(ctx, pool, seed.Options{Env: "development"})
	require.NoError(t, err)
	require.Equal(t, []string{"reference/0001_ref.sql"}, applied)
	require.True(t, slugExists(t, pool, "ref-marker"))
	require.False(t, slugExists(t, pool, "dev-marker"), "dev seeds must not run unless enabled")

	applied, err = r.Apply(ctx, pool, seed.Options{Env: "development", Dev: true})
	require.NoError(t, err)
	require.Equal(t, []string{
		"reference/0001_ref.sql",
		"dev/0001_dev.sql",
		"dev/0002_dev_depends.sql",
	}, applied, "reference runs first, then dev, each in lexical order")

	var desc sql.NullString
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT description FROM categories WHERE slug = 'dev-marker'`).Scan(&desc))
	require.Equal(t, "touched", desc.String)
}

func TestProductionAppliesReferenceButRefusesDev(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	r := &seed.Runner{FS: markerFS()}

	_, err := r.Apply(ctx, pool, seed.Options{Env: "production", Dev: true})
	require.ErrorIs(t, err, seed.ErrDevSeedNotAllowed)
	require.False(t, slugExists(t, pool, "ref-marker"), "refusal happens before any write")

	_, err = r.Apply(ctx, pool, seed.Options{Env: "production"})
	require.NoError(t, err)
	require.True(t, slugExists(t, pool, "ref-marker"))
}

func TestFailingSeedFileRollsBackEverything(t *testing.T) {
	pool := provisionMigratedDB(t)
	r := &seed.Runner{FS: fstest.MapFS{
		"reference/0001_ok.sql":     {Data: []byte(`INSERT INTO categories (slug, name) VALUES ('ok', 'OK');`)},
		"reference/0002_broken.sql": {Data: []byte(`INSERT INTO no_such_table VALUES (1);`)},
	}}

	_, err := r.Apply(context.Background(), pool, seed.Options{Env: "development"})
	require.ErrorContains(t, err, "reference/0002_broken.sql")
	require.False(t, slugExists(t, pool, "ok"), "seeds apply in one transaction")
}

func TestEmbeddedSeedsApplyCleanlyWithDevEnabled(t *testing.T) {
	pool := provisionMigratedDB(t)
	_, err := seed.Apply(context.Background(), pool, seed.Options{Env: "development", Dev: true})
	require.NoError(t, err)
}
