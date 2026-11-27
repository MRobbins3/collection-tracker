//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/migrations"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// admin DB URL; we connect here to create/drop disposable per-test databases
// so integration tests never touch the developer-facing `collection` DB.
// The host comes from the env — `db:5432` inside docker-compose, `localhost`
// in GitHub Actions with a Postgres service container.
func adminURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_ADMIN_DATABASE_URL")
	if url == "" {
		url = "postgres://collection:collection@db:5432/postgres?sslmode=disable"
	}
	return url
}

// testDBURL returns a connection string to a database named dbName on the
// same host/port as adminURL — so the tests work both in docker-compose
// (host=db) and on a GitHub Actions runner (host=localhost).
func testDBURL(t *testing.T, dbName string) string {
	t.Helper()
	return strings.Replace(adminURL(t), "/postgres?", "/"+dbName+"?", 1)
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

	url := testDBURL(t, dbName)
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
		`INSERT INTO users (google_sub, email, display_name, handle) VALUES ('s','e@x','T','tester') RETURNING id`,
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
		`INSERT INTO users (google_sub, email, display_name, handle) VALUES ($1, $2, $3, $4) RETURNING id`,
		"google-sub-1", "user@example.com", "Test User", "test_user",
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
		`INSERT INTO users (google_sub, email, display_name, handle) VALUES ('g','e@x','T','tester') RETURNING id`,
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

// TestProfilesMigrationBackfillsExistingUsers runs migration 0003 against a
// database that already has users, as production will, and checks every row
// gets a valid, unique handle and the privacy-respecting defaults. Then it
// rolls back and forward again.
func TestProfilesMigrationBackfillsExistingUsers(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	require.NoError(t, migrations.UpTo(ctx, dbURL, 2))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Inserted in created_at order; earlier rows win a contested handle.
	existing := []struct{ sub, email, name, wantHandle string }{
		{"s1", "liam.johnson@example.com", "Liam Johnson", "liam_johnson"},
		{"s2", "liam.j@example.com", "Liam Johnson", "liam_johnson2"},
		{"s3", "lj@example.com", "LIAM   johnson", "liam_johnson3"},
		{"s4", "jg@example.com", "José García", "jose_garcia"},
		{"s5", "fan.club@example.com", "42", "collector"}, // never the email
		{"s6", "r2@example.com", "R2", "collector2"},
		{"s6b", "watanabe.k@example.com", "渡辺", "collector3"},
		{"s7", "boss@example.com", "Admin", "admin2"},
		{"s8", "x@example.com", "3PO Fan", "u_3po_fan"},
		{"s9", "e@example.com", "An Extremely Long Display Name Indeed", "an_extremely_long_displa"},
	}
	for i, u := range existing {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (google_sub, email, display_name, created_at)
			 VALUES ($1, $2, $3, now() - make_interval(mins => $4))`,
			u.sub, u.email, u.name, len(existing)-i)
		require.NoError(t, err)
	}
	var userWithData string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM users WHERE google_sub='s1'`).Scan(&userWithData))
	var categoryID string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name) VALUES ('c','C') RETURNING id`).Scan(&categoryID))
	_, err = db.ExecContext(ctx,
		`INSERT INTO collections (user_id, category_id, name) VALUES ($1, $2, 'Kept')`, userWithData, categoryID)
	require.NoError(t, err)

	require.NoError(t, migrations.Up(ctx, dbURL), "0003 must apply on a DB with existing rows")

	for _, u := range existing {
		var handle, bio, vis string
		var acceptFriends, acceptFollows bool
		require.NoError(t, db.QueryRowContext(ctx, `
			SELECT handle, bio, collection_visibility, accept_friend_requests, accept_follow_requests
			FROM users WHERE google_sub = $1`, u.sub,
		).Scan(&handle, &bio, &vis, &acceptFriends, &acceptFollows))
		require.Equal(t, u.wantHandle, handle, u.name)
		require.Empty(t, bio)
		require.Equal(t, "friends_only", vis)
		require.True(t, acceptFriends)
		require.True(t, acceptFollows)
	}

	// The database enforces the rules too.
	_, err = db.ExecContext(ctx, `UPDATE users SET handle = 'Liam_Johnson4' WHERE google_sub = 's1'`)
	require.Error(t, err, "uppercase handles violate the format check")
	_, err = db.ExecContext(ctx, `UPDATE users SET handle = 'jose_garcia' WHERE google_sub = 's1'`)
	require.Error(t, err, "handles are unique")
	_, err = db.ExecContext(ctx, `UPDATE users SET collection_visibility = 'everyone' WHERE google_sub = 's1'`)
	require.Error(t, err, "visibility is a closed set")
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (google_sub, email, display_name) VALUES ('s10', 'n@example.com', 'No Handle')`)
	require.Error(t, err, "handle is required once the backfill has run")

	// Reversible: down drops the new columns and keeps the rows and their data.
	require.NoError(t, migrations.DownTo(ctx, dbURL, 2))
	var hasHandle bool
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns
		               WHERE table_name = 'users' AND column_name = 'handle')`).Scan(&hasHandle))
	require.False(t, hasHandle)
	var users, collections int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&users))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM collections`).Scan(&collections))
	require.Equal(t, len(existing), users)
	require.Equal(t, 1, collections)

	require.NoError(t, migrations.Up(ctx, dbURL), "and forward again")
}

// Catalog entries are unique per category on (normalized name, normalized
// identifier), so the seed and later submissions can't create near-duplicates
// that differ only in case or spacing.
func TestCatalogEntriesUniquePerCategoryOnNormalizedKey(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, migrations.Up(ctx, dbURL))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var lego, books string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name, catalog_identifier) VALUES ('lego','Lego','set_number') RETURNING id`,
	).Scan(&lego))
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name) VALUES ('books','Books') RETURNING id`,
	).Scan(&books))

	insert := func(categoryID, name string, identifier any) error {
		_, err := db.ExecContext(ctx,
			`INSERT INTO catalog_entries (category_id, name, identifier) VALUES ($1, $2, $3)`,
			categoryID, name, identifier)
		return err
	}

	require.NoError(t, insert(lego, "Millennium Falcon", "75192"))
	require.Error(t, insert(lego, "  millennium   FALCON ", " 75192"),
		"same name and identifier after trim/collapse/lowercase is a duplicate")
	require.NoError(t, insert(lego, "Millennium Falcon", "10179"), "a different set number is a different entry")
	require.NoError(t, insert(lego, "Millennium Falcon", nil), "no identifier is its own key")
	require.Error(t, insert(lego, "MILLENNIUM FALCON", nil), "and is unique too")
	require.NoError(t, insert(books, "Millennium Falcon", "75192"), "uniqueness is per category")
	require.Error(t, insert(lego, "   ", nil), "blank names are rejected")

	var nameKey, identifierKey string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT name_key, identifier_key FROM catalog_entries WHERE identifier = '10179'`,
	).Scan(&nameKey, &identifierKey))
	require.Equal(t, "millennium falcon", nameKey)
	require.Equal(t, "10179", identifierKey)

	require.NoError(t, migrations.DownTo(ctx, dbURL, 4), "the key migration rolls back")
}

// The generated keys normalize Unicode whitespace exactly like the API's
// textnorm.Key, so a name pasted with a non-breaking space can't sneak past
// the natural key.
func TestCatalogKeysMatchTextnormForUnicodeSpaces(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, migrations.Up(ctx, dbURL))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var cat string
	require.NoError(t, db.QueryRowContext(ctx,
		`INSERT INTO categories (slug, name) VALUES ('vinyl','Vinyl') RETURNING id`).Scan(&cat))

	for _, name := range []string{
		"\u00a0Abbey\u00a0Road\u3000",
		"\tKind  of\u2009Blue\n",
		"The\u202fDark Side",
	} {
		var nameKey, identifierKey string
		require.NoError(t, db.QueryRowContext(ctx, `
			INSERT INTO catalog_entries (category_id, name, identifier) VALUES ($1, $2, $2)
			RETURNING name_key, identifier_key`, cat, name).Scan(&nameKey, &identifierKey), name)
		require.Equal(t, textnorm.Key(name), nameKey, "%q", name)
		require.Equal(t, textnorm.Key(name), identifierKey, "%q", name)
	}

	_, err = db.ExecContext(ctx,
		`INSERT INTO catalog_entries (category_id, name) VALUES ($1, 'Abbey Road')`, cat)
	require.NoError(t, err, "no identifier: a different key from the NBSP entry above")
	_, err = db.ExecContext(ctx,
		`INSERT INTO catalog_entries (category_id, name, identifier) VALUES ($1, 'abbey road', 'Abbey Road')`, cat)
	require.Error(t, err, "same key as the NBSP entry once normalized")
	_, err = db.ExecContext(ctx,
		`INSERT INTO catalog_entries (category_id, name) VALUES ($1, E'\u00a0\u3000')`, cat)
	require.Error(t, err, "a name of only Unicode spaces is blank")
}

// Catalog search filters with LIKE '%…%' on the normalized keys; the
// trigram indexes must be on those columns. enable_seqscan=off makes the
// planner use an index when one applies, even on a tiny table.
func TestCatalogSearchUsesTrigramIndexesOnKeys(t *testing.T) {
	dbURL, cleanup := provisionDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, migrations.Up(ctx, dbURL))

	db, err := sql.Open("pgx", dbURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// A few thousand rows and fresh stats, so the plan reflects a real
	// category rather than an empty table.
	_, err = conn.ExecContext(ctx, `
		WITH cat AS (INSERT INTO categories (slug, name) VALUES ('lego','Lego') RETURNING id)
		INSERT INTO catalog_entries (category_id, name, identifier, status)
		SELECT cat.id, 'Set ' || g, g::text, 'approved' FROM cat, generate_series(1, 3000) g`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `ANALYZE catalog_entries`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `SET enable_seqscan = off`)
	require.NoError(t, err)
	explain := func(query string) string {
		rows, err := conn.QueryContext(ctx, "EXPLAIN "+query)
		require.NoError(t, err)
		defer rows.Close()
		var plan strings.Builder
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			plan.WriteString(line + "\n")
		}
		require.NoError(t, rows.Err())
		return plan.String()
	}
	require.Contains(t, explain(`SELECT id FROM catalog_entries WHERE name_key LIKE '%falcon%'`),
		"catalog_entries_name_key_trgm")
	require.Contains(t, explain(`SELECT id FROM catalog_entries WHERE identifier_key LIKE '%7519%'`),
		"catalog_entries_identifier_key_trgm")
}
