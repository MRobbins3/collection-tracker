//go:build integration

package seed_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
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

// The eight curated categories, ported verbatim from the retired
// categories.json (parity was proven by a test before the JSON was deleted).
func TestReferenceSeedsProduceCuratedCategories(t *testing.T) {
	pool := provisionMigratedDB(t)
	_, err := seed.Apply(context.Background(), pool, seed.Options{Env: "development"})
	require.NoError(t, err)

	want := map[string]struct {
		name, description string
		properties        []string
	}{
		"lego-sets":     {"Lego Sets", "Official Lego sets, minifigures, and themed collections.", []string{"set_number", "theme", "piece_count"}},
		"funko-pops":    {"Funko Pops", "Funko Pop! vinyl figures, chases, and exclusives.", []string{"series", "number", "variant"}},
		"coins":         {"Coins", "Circulating and commemorative coins from around the world.", []string{"year", "country", "denomination", "mint_mark"}},
		"stamps":        {"Stamps", "Postal stamps, commemoratives, and first-day covers.", []string{"year", "country", "catalog_id"}},
		"trading-cards": {"Trading Cards", "Sports cards, TCGs, and collectible card games.", []string{"game", "set_name", "card_number", "rarity"}},
		"plants":        {"Plants", "Houseplants, succulents, and garden specimens.", []string{"species", "pot_size", "acquired_from"}},
		"books":         {"Books", "Physical books — fiction, non-fiction, first editions, signed copies.", []string{"author", "isbn", "edition"}},
		"vinyl-records": {"Vinyl Records", "LPs, EPs, 7-inches, and box sets.", []string{"artist", "album", "year", "label", "pressing"}},
	}

	got := categoriesBySlug(t, pool)
	require.Len(t, got, len(want))
	for slug, w := range want {
		g, ok := got[slug]
		require.True(t, ok, "missing category %s", slug)
		require.Equal(t, w.name, g.Name, slug)
		require.NotNil(t, g.Description, slug)
		require.Equal(t, w.description, *g.Description, slug)

		require.Equal(t, "object", g.Schema["type"], slug)
		require.Equal(t, false, g.Schema["additionalProperties"], slug)
		props, ok := g.Schema["properties"].(map[string]any)
		require.True(t, ok, "%s: properties must be an object", slug)
		require.Len(t, props, len(w.properties), slug)
		for _, key := range w.properties {
			prop, ok := props[key].(map[string]any)
			require.True(t, ok, "%s: missing property %s", slug, key)
			require.NotEmpty(t, prop["type"], "%s.%s needs a type", slug, key)
			require.NotEmpty(t, prop["title"], "%s.%s needs a human title (no raw keys in UI)", slug, key)
		}
	}

	lego := got["lego-sets"].Schema["properties"].(map[string]any)["set_number"].(map[string]any)
	require.Equal(t, map[string]any{
		"type":        "string",
		"title":       "Set number",
		"description": "The number printed on the box, e.g. 75192.",
	}, lego, "spot-check one property verbatim")
}

// tableSnapshot is, per table in schema public (bar goose's bookkeeping),
// the row count and every row's xmin. xmin changes whenever a row is
// rewritten, so equal snapshots mean a run inserted, deleted, and updated
// nothing.
type tableSnapshot struct {
	Count int64
	Xmins []string
}

func snapshotTables(t *testing.T, pool *pgxpool.Pool) map[string]tableSnapshot {
	t.Helper()
	ctx := context.Background()

	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		  AND table_name <> 'goose_db_version'`)
	require.NoError(t, err)
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	require.NotEmpty(t, tables)

	out := map[string]tableSnapshot{}
	for _, tbl := range tables {
		ident := pgx.Identifier{tbl}.Sanitize()
		var snap tableSnapshot
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT count(*), coalesce(array_agg(xmin::text ORDER BY xmin::text), '{}') FROM `+ident,
		).Scan(&snap.Count, &snap.Xmins), tbl)
		out[tbl] = snap
	}
	return out
}

// Guards every seed file, present and future: a second run must not add,
// remove, or rewrite a single row in any table.
func TestSeedsAreIdempotentAcrossAllTables(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	opts := seed.Options{Env: "development", Dev: true}

	_, err := seed.Apply(ctx, pool, opts)
	require.NoError(t, err)
	first := snapshotTables(t, pool)
	require.EqualValues(t, 8, first["categories"].Count)
	firstCats := categoriesBySlug(t, pool)

	_, err = seed.Apply(ctx, pool, opts)
	require.NoError(t, err)
	require.Equal(t, first, snapshotTables(t, pool),
		"a second run must not insert, delete, or rewrite rows (use ON CONFLICT with an IS DISTINCT FROM guard)")
	require.Equal(t, firstCats, categoriesBySlug(t, pool))
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
