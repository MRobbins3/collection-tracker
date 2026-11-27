//go:build integration

package seed_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
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

// The dev set gives the personas friends and followers so every visibility
// level is demoable (ADR 0014): Noah (Friends only) is Liam's friend, Liam
// follows Amelia (Followers & friends), and Emma has a request to answer.
// Olivia has no ties, so she sees what a stranger sees.
func TestDevSeedsRelateThePersonas(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	users := store.NewUsers(pool)
	rels := store.NewRelationships(pool)
	id := func(handle string) string {
		u, err := users.GetByGoogleSub(ctx, "dev:"+handle)
		require.NoError(t, err)
		return u.ID
	}
	sees := func(viewer, owner string) bool {
		rel, err := rels.Between(ctx, id(viewer), id(owner))
		require.NoError(t, err)
		p, err := users.GetProfile(ctx, id(owner))
		require.NoError(t, err)
		return privacy.CanViewCollections(p.CollectionVisibility, rel)
	}

	require.True(t, sees("liam", "noah"), "friends see Friends only")
	require.False(t, sees("olivia", "noah"))
	require.True(t, sees("liam", "amelia"), "an approved follower sees Followers & friends")
	require.False(t, sees("olivia", "amelia"))
	require.False(t, sees("amelia", "emma"), "a pending follow request unlocks nothing")

	followers, err := rels.ListFollowers(ctx, id("liam"))
	require.NoError(t, err)
	require.Len(t, followers, 1)
	require.Equal(t, "amelia", followers[0].Handle)

	reqs, err := rels.ListRequests(ctx, id("emma"))
	require.NoError(t, err)
	require.Len(t, reqs.FollowIncoming, 1)
	require.Equal(t, "amelia", reqs.FollowIncoming[0].Handle)

	friends, err := rels.ListFriends(ctx, id("olivia"))
	require.NoError(t, err)
	require.Empty(t, friends, "Olivia stays a stranger to everyone")
	n, err := rels.IncomingRequestCount(ctx, id("olivia"))
	require.NoError(t, err)
	require.Zero(t, n)
}
