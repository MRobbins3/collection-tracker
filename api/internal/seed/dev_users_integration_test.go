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
		{"dev:noah", "Noah Williams", "noah.williams@example.com", 2, 6},
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

// Sample collections and items are DO NOTHING on conflict in every dev seed
// file: once a row exists, a re-run leaves it exactly as the app left it,
// created_at included.
func TestDevSeedRerunLeavesSampleItemsAlone(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	opts := seed.Options{Env: "test", Dev: true}
	_, err := seed.Apply(ctx, pool, opts)
	require.NoError(t, err)

	// One item from 0001 and one from 0004.
	const fromFirst, fromCompare = "d0000000-0000-4000-8000-000000000201", "d0000000-0000-4000-8000-000000000241"
	for _, id := range []string{fromFirst, fromCompare} {
		_, err = pool.Exec(ctx,
			`UPDATE items SET name = 'Edited', created_at = '2020-01-01 00:00:00+00' WHERE id = $1`, id)
		require.NoError(t, err)
	}

	_, err = seed.Apply(ctx, pool, opts)
	require.NoError(t, err)
	for _, id := range []string{fromFirst, fromCompare} {
		var name string
		var year int
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT name, extract(year FROM created_at)::int FROM items WHERE id = $1`, id).Scan(&name, &year))
		require.Equal(t, "Edited", name, id)
		require.Equal(t, 2020, year, "created_at is left alone on a re-run (%s)", id)
	}
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

// Liam and Noah are friends with overlapping Lego sets, so comparing their
// collections shows all four buckets (0004_compare_sample.sql).
func TestDevSeedsMakeLiamAndNoahComparable(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	users := store.NewUsers(pool)
	liam, err := users.GetByGoogleSub(ctx, "dev:liam")
	require.NoError(t, err)
	noah, err := users.GetByGoogleSub(ctx, "dev:noah")
	require.NoError(t, err)

	cmp := store.NewCompare(pool)
	names := map[store.CompareBucket][]string{}
	var counts store.CompareCounts
	for _, b := range []store.CompareBucket{store.BucketBoth, store.BucketMineOnly, store.BucketTheirsOnly, store.BucketNeither} {
		res, err := cmp.Compare(ctx, store.CompareQuery{
			ViewerID: liam.ID, OtherID: noah.ID, CategorySlug: "lego-sets", Bucket: b, Limit: 50,
		})
		require.NoError(t, err)
		counts = res.Counts
		for _, r := range res.Items {
			names[b] = append(names[b], r.Name)
		}
	}
	require.Equal(t, store.CompareCounts{Both: 2, MineOnly: 2, TheirsOnly: 2, Neither: 3}, counts)
	require.Equal(t, map[store.CompareBucket][]string{
		store.BucketBoth:       {"Galaxy Explorer", "Tree House"},
		store.BucketMineOnly:   {"Lamborghini Sián FKP 37", "Millennium Falcon"},
		store.BucketTheirsOnly: {"NASA Apollo Saturn V", "Typewriter"},
		store.BucketNeither:    {"AT-AT", "Colosseum", "Titanic"},
	}, names)

	var linked int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT count(*) FROM items WHERE collection_id = 'd0000000-0000-4000-8000-000000000105'
		AND catalog_entry_id IS NOT NULL`).Scan(&linked))
	require.Equal(t, 2, linked, "Tree House and Typewriter are linked to the catalog")
}
