//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/migrations"
)

// admin DB URL; we connect here to create/drop disposable per-test databases
// so integration tests never touch the developer-facing `collection` DB.
func adminURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_ADMIN_DATABASE_URL")
	if url == "" {
		url = "postgres://collection:collection@db:5432/postgres?sslmode=disable"
	}
	return url
}

// provisionDB creates a fresh DB for a single test and returns its URL plus
// a cleanup function that drops it. Names are lowercased + sanitized so they
// survive Postgres identifier rules without quoting gymnastics.
func provisionDB(t *testing.T, name string) (string, func()) {
	t.Helper()

	admin, err := sql.Open("pgx", adminURL(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, admin.PingContext(ctx))

	dbName := "ct_test_" + strings.ToLower(strings.ReplaceAll(name, "/", "_"))
	_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName)
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+dbName)
	require.NoError(t, err, "create db %s", dbName)

	url := fmt.Sprintf("postgres://collection:collection@db:5432/%s?sslmode=disable", dbName)
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
	}
	return url, cleanup
}

func TestMigrationsUpAndDown(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, migrations.Up(ctx, dbURL), "up should succeed")

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	expectedTables := []string{"users", "categories", "collections", "items", "catalog_entries"}
	for _, tbl := range expectedTables {
		var exists bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`,
			tbl,
		).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist after Up", tbl)
	}

	// items should have the catalog_entry_id column (nullable FK).
	var catalogCol bool
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'items' AND column_name = 'catalog_entry_id'
		)
	`).Scan(&catalogCol))
	require.True(t, catalogCol, "items.catalog_entry_id should exist")

	require.NoError(t, migrations.DownTo(ctx, dbURL, 0), "down-to 0 should succeed")

	for _, tbl := range expectedTables {
		var exists bool
		err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name=$1)`,
			tbl,
		).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "table %s should be gone after Down", tbl)
	}
}

func TestCatalogEntrySetNullOnDeletePreservesItems(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, migrations.Up(ctx, dbURL))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Seed the bare minimum: user, category, collection, catalog entry, item.
	var userID, categoryID, collectionID, catalogID, itemID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users (google_sub, email, display_name) VALUES ('s','e@x','T') RETURNING id`,
	).Scan(&userID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name) VALUES ('c','C') RETURNING id`,
	).Scan(&categoryID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO collections (user_id, category_id, name) VALUES ($1,$2,'n') RETURNING id`,
		userID, categoryID,
	).Scan(&collectionID))
	require.NoError(t, db.QueryRowContext(ctx, `
		INSERT INTO catalog_entries (category_id, name, status, source)
		VALUES ($1, 'Canonical Thing', 'approved', 'seed') RETURNING id
	`, categoryID).Scan(&catalogID))
	require.NoError(t, db.QueryRowContext(ctx, `
		INSERT INTO items (collection_id, name, quantity, catalog_entry_id)
		VALUES ($1, 'My copy', 1, $2) RETURNING id
	`, collectionID, catalogID).Scan(&itemID))

	// Deleting the catalog entry should NOT cascade into items — it should
	// convert the item to free-form by nulling its catalog_entry_id.
	_, err = db.ExecContext(ctx, `DELETE FROM catalog_entries WHERE id = $1`, catalogID)
	require.NoError(t, err)

	var lingering sql.NullString
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT catalog_entry_id FROM items WHERE id = $1`, itemID,
	).Scan(&lingering))
	require.False(t, lingering.Valid, "item.catalog_entry_id should be NULL after catalog delete")

	var stillThere int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM items WHERE id = $1`, itemID,
	).Scan(&stillThere))
	require.Equal(t, 1, stillThere, "item should still exist as free-form after catalog delete")
}

func TestSchemaRoundtripsJSONBAttributes(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, migrations.Up(ctx, dbURL))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var userID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users (google_sub, email, display_name) VALUES ($1, $2, $3) RETURNING id`,
		"google-sub-1", "user@example.com", "Test User",
	).Scan(&userID))

	var categoryID string
	schema := `{"type":"object","properties":{"set_number":{"type":"string"},"piece_count":{"type":"integer"}}}`
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name, description, attribute_schema) VALUES ($1, $2, $3, $4::jsonb) RETURNING id`,
		"lego-sets", "Lego Sets", "Lego building sets.", schema,
	).Scan(&categoryID))

	var collectionID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO collections (user_id, category_id, name) VALUES ($1, $2, $3) RETURNING id`,
		userID, categoryID, "My Star Wars Lego",
	).Scan(&collectionID))

	attrs := map[string]any{"set_number": "75192", "piece_count": 7541}
	attrsJSON, err := json.Marshal(attrs)
	require.NoError(t, err)

	var itemID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO items (collection_id, name, quantity, condition, attributes)
		 VALUES ($1, $2, $3, $4, $5::jsonb) RETURNING id`,
		collectionID, "Millennium Falcon UCS", 1, "New, sealed", string(attrsJSON),
	).Scan(&itemID))

	var gotName string
	var gotQty int
	var gotAttrsRaw []byte
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT name, quantity, attributes FROM items WHERE id=$1`, itemID,
	).Scan(&gotName, &gotQty, &gotAttrsRaw))

	require.Equal(t, "Millennium Falcon UCS", gotName)
	require.Equal(t, 1, gotQty)

	var gotAttrs map[string]any
	require.NoError(t, json.Unmarshal(gotAttrsRaw, &gotAttrs))
	require.Equal(t, "75192", gotAttrs["set_number"])
	require.EqualValues(t, 7541, gotAttrs["piece_count"])
}

func TestQuantityCheckConstraint(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	require.NoError(t, migrations.Up(ctx, dbURL))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var userID, categoryID, collectionID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO users (google_sub, email, display_name) VALUES ('g','e@x','T') RETURNING id`,
	).Scan(&userID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name) VALUES ('c','C') RETURNING id`,
	).Scan(&categoryID))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO collections (user_id, category_id, name) VALUES ($1,$2,'n') RETURNING id`,
		userID, categoryID,
	).Scan(&collectionID))

	_, err = db.ExecContext(ctx,
		`INSERT INTO items (collection_id, name, quantity) VALUES ($1, 'bad', -1)`,
		collectionID,
	)
	require.Error(t, err, "negative quantity should violate CHECK constraint")
}
