//go:build integration

package seed_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
)

type categoryRow struct {
	ID          string
	Slug        string
	Name        string
	Description *string
	Schema      map[string]any
}

func categoriesBySlug(t *testing.T, pool *pgxpool.Pool) map[string]categoryRow {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT id::text, slug, name, description, attribute_schema FROM categories`)
	require.NoError(t, err)
	defer rows.Close()

	out := map[string]categoryRow{}
	for rows.Next() {
		var c categoryRow
		require.NoError(t, rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Description, &c.Schema))
		out[c.Slug] = c
	}
	require.NoError(t, rows.Err())
	return out
}

// Parity with the JSON file the SQL seeds replace. Removed with the JSON.
func TestReferenceSeedsMatchLegacyCategoriesJSON(t *testing.T) {
	pool := provisionMigratedDB(t)
	_, err := seed.Apply(context.Background(), pool, seed.Options{Env: "development"})
	require.NoError(t, err)

	raw, err := os.ReadFile("data/categories.json")
	require.NoError(t, err)
	var want []struct {
		Slug            string         `json:"slug"`
		Name            string         `json:"name"`
		Description     *string        `json:"description"`
		AttributeSchema map[string]any `json:"attribute_schema"`
	}
	require.NoError(t, json.Unmarshal(raw, &want))

	got := categoriesBySlug(t, pool)
	require.Len(t, got, len(want))
	for _, w := range want {
		g, ok := got[w.Slug]
		require.True(t, ok, "missing category %s", w.Slug)
		require.Equal(t, w.Name, g.Name, w.Slug)
		require.Equal(t, w.Description, g.Description, w.Slug)
		require.Equal(t, w.AttributeSchema, g.Schema, w.Slug)
	}
}

func TestReferenceSeedsAreIdempotent(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()

	_, err := seed.Apply(ctx, pool, seed.Options{Env: "development", Dev: true})
	require.NoError(t, err)
	first := categoriesBySlug(t, pool)
	require.Len(t, first, 8)

	_, err = seed.Apply(ctx, pool, seed.Options{Env: "development", Dev: true})
	require.NoError(t, err)
	require.Equal(t, first, categoriesBySlug(t, pool),
		"a second run must not duplicate, delete-and-reinsert (new ids), or change rows")
}

func TestReferenceSeedsRestoreHandEditedCategory(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()

	_, err := seed.Apply(ctx, pool, seed.Options{Env: "development"})
	require.NoError(t, err)
	want := categoriesBySlug(t, pool)["coins"]

	_, err = pool.Exec(ctx,
		`UPDATE categories SET name = 'Drifted', attribute_schema = '{}' WHERE slug = 'coins'`)
	require.NoError(t, err)

	_, err = seed.Apply(ctx, pool, seed.Options{Env: "development"})
	require.NoError(t, err)
	require.Equal(t, want, categoriesBySlug(t, pool)["coins"],
		"the SQL file is the source of truth; re-running refreshes drifted rows in place")
}
