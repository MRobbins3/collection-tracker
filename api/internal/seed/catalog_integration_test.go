//go:build integration

package seed_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
)

// The sample catalog is dev data (the owner's real dataset is pending): it
// must not appear in production, where only the reference set runs.
func TestSampleCatalogIsDevOnly(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "production"})
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM catalog_entries`).Scan(&n))
	require.Zero(t, n)
}

// Seeds bypass the API, so check every sample entry here: approved seed
// entries, attributes valid for the category, and identifier copied from the
// category's identifier attribute.
func TestSampleCatalogEntriesAreApprovedAndValid(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	rows, err := pool.Query(ctx, `
		SELECT cat.slug, ce.name, ce.source, ce.status, ce.approved_at IS NOT NULL,
		       cat.attribute_schema, ce.attributes,
		       ce.identifier, ce.attributes ->> cat.catalog_identifier
		FROM catalog_entries ce
		JOIN categories cat ON cat.id = ce.category_id`)
	require.NoError(t, err)
	defer rows.Close()

	perCategory := map[string]int{}
	for rows.Next() {
		var slug, name, source, status string
		var approved bool
		var schema, attrs json.RawMessage
		var identifier, fromAttrs *string
		require.NoError(t, rows.Scan(&slug, &name, &source, &status, &approved,
			&schema, &attrs, &identifier, &fromAttrs))
		require.Equal(t, "seed", source, name)
		require.Equal(t, "approved", status, name)
		require.True(t, approved, "%s: approved_at is set", name)
		require.NoError(t, category.ValidateAttributes(schema, attrs), name)
		require.Equal(t, fromAttrs, identifier, "%s: identifier mirrors the identifier attribute", name)
		perCategory[slug]++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]int{"lego-sets": 8, "vinyl-records": 6, "coins": 6}, perCategory)
}

// A user's pending suggestion that shares a sample entry's key must stay
// pending (its moderation request decides it); the seed must not approve it.
func TestSampleCatalogLeavesPendingSuggestionsAlone(t *testing.T) {
	pool := provisionMigratedDB(t)
	ctx := context.Background()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test"})
	require.NoError(t, err)
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO catalog_entries (category_id, name, identifier, attributes, source, status)
		SELECT id, 'millennium falcon', '75192', '{"set_number":"75192"}', 'user_submitted', 'pending'
		FROM categories WHERE slug = 'lego-sets' RETURNING id`).Scan(&id))

	_, err = seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)

	var source, status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT source, status FROM catalog_entries WHERE id = $1`, id).Scan(&source, &status))
	require.Equal(t, "user_submitted", source)
	require.Equal(t, "pending", status)
}
