//go:build integration

package seed_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
)

// The dev set seeds the sign-in personas (ADR 0008). Each one is keyed on a
// 'dev:'-prefixed google_sub so it can never collide with a real Google
// account, and each covers a scenario later features build on.
func TestDevSeedsCreatePersonas(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	rows, err := pool.Query(ctx, `
		SELECT u.google_sub, u.display_name, u.email,
		       count(DISTINCT c.id), count(i.id)
		FROM users u
		LEFT JOIN collections c ON c.user_id = u.id
		LEFT JOIN items i ON i.collection_id = c.id
		GROUP BY u.id ORDER BY u.google_sub`)
	require.NoError(t, err)
	type persona struct {
		Sub, Name, Email   string
		Collections, Items int
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[persona])
	require.NoError(t, err)

	require.Equal(t, []persona{
		{"dev:amelia", "Amelia Brown", "amelia.brown@example.com", 1, 2},
		{"dev:emma", "Emma Garcia", "emma.garcia@example.com", 0, 0},
		{"dev:liam", "Liam Johnson", "liam.johnson@example.com", 2, 6},
		{"dev:noah", "Noah Williams", "noah.williams@example.com", 1, 2},
		{"dev:olivia", "Olivia Martinez", "olivia.martinez@example.com", 0, 0},
	}, got)
}

// Olivia is the dev admin (ADR 0012); every other persona has no role.
func TestDevSeedsMakeOliviaTheOnlyAdmin(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	rows, err := pool.Query(ctx, `SELECT google_sub FROM users WHERE role = 'admin' ORDER BY google_sub`)
	require.NoError(t, err)
	admins, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	require.Equal(t, []string{"dev:olivia"}, admins)
}

// Seeds bypass the API's attribute validation, so check every seeded item
// against its category schema here; otherwise a typo would only surface
// when someone edits the item in the app.
func TestDevSeedItemsMatchTheirCategorySchema(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	rows, err := pool.Query(ctx, `
		SELECT i.name, cat.attribute_schema, i.attributes
		FROM items i
		JOIN collections c ON c.id = i.collection_id
		JOIN categories cat ON cat.id = c.category_id`)
	require.NoError(t, err)
	defer rows.Close()

	n := 0
	for rows.Next() {
		var name string
		var schema, attrs json.RawMessage
		require.NoError(t, rows.Scan(&name, &schema, &attrs))
		require.NoError(t, category.ValidateAttributes(schema, attrs), name)
		n++
	}
	require.NoError(t, rows.Err())
	require.Positive(t, n, "expected seeded items")
}
