//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func TestItemsSearchForUser(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)

	colls := store.NewCollections(pool)
	aliceLego, err := colls.Create(ctx, alice.ID, "lego-sets", "Lego Shelf")
	require.NoError(t, err)
	aliceVinyl, err := colls.Create(ctx, alice.ID, "vinyl-records", "Records")
	require.NoError(t, err)
	bobLego, err := colls.Create(ctx, bob.ID, "lego-sets", "Bob's Lego")
	require.NoError(t, err)

	items := store.NewItems(pool)
	add := func(collID, userID, name string, attrs map[string]any) store.Item {
		t.Helper()
		raw, err := json.Marshal(attrs)
		require.NoError(t, err)
		it, err := items.Create(ctx, collID, userID, store.ItemInput{Name: name, Quantity: 1, Attributes: raw})
		require.NoError(t, err)
		return it
	}
	falcon := add(aliceLego.ID, alice.ID, "Millennium Falcon", map[string]any{"set_number": "75192", "theme": "Star Wars"})
	add(aliceLego.ID, alice.ID, "Falcon Mini Build", map[string]any{"set_number": "30212"})
	add(aliceLego.ID, alice.ID, "Tree House", map[string]any{"set_number": "21318", "theme": "Ideas"})
	abbey := add(aliceVinyl.ID, alice.ID, "Abbey Road", map[string]any{"artist": "The Beatles", "year": 1969})
	add(aliceLego.ID, alice.ID, "100% Brick_Box", map[string]any{})
	add(bobLego.ID, bob.ID, "Millennium Falcon", map[string]any{"set_number": "75192"})

	names := func(rs []store.ItemSearchResult) []string {
		out := make([]string, 0, len(rs))
		for _, r := range rs {
			out = append(out, r.Name)
		}
		return out
	}

	t.Run("returns only the caller's items, with collection and category labels", func(t *testing.T) {
		got, more, err := items.SearchForUser(ctx, alice.ID, "millennium", 50)
		require.NoError(t, err)
		require.False(t, more)
		require.Len(t, got, 1)
		r := got[0]
		require.Equal(t, falcon.ID, r.ID)
		require.Equal(t, aliceLego.ID, r.CollectionID)
		require.Equal(t, "Lego Shelf", r.CollectionName)
		require.Equal(t, "lego-sets", r.CategorySlug)
		require.Equal(t, "Lego Sets", r.CategoryName)
		require.Nil(t, r.MatchedField, "name matches don't report an attribute")

		bobs, _, err := items.SearchForUser(ctx, bob.ID, "millennium", 50)
		require.NoError(t, err)
		require.Len(t, bobs, 1)
		require.NotEqual(t, falcon.ID, bobs[0].ID)
	})

	t.Run("substring matches are case-insensitive and rank prefix matches first", func(t *testing.T) {
		got, _, err := items.SearchForUser(ctx, alice.ID, "FALCON", 50)
		require.NoError(t, err)
		require.Equal(t, []string{"Falcon Mini Build", "Millennium Falcon"}, names(got))
	})

	t.Run("tolerates a typo via trigram word similarity", func(t *testing.T) {
		got, _, err := items.SearchForUser(ctx, alice.ID, "falcom", 50)
		require.NoError(t, err)
		require.Contains(t, names(got), "Millennium Falcon")
	})

	t.Run("matches attribute values and reports the human label, after name matches", func(t *testing.T) {
		got, _, err := items.SearchForUser(ctx, alice.ID, "beatles", 50)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, abbey.ID, got[0].ID)
		require.NotNil(t, got[0].MatchedField)
		require.Equal(t, "Artist", got[0].MatchedField.Label)
		require.Equal(t, "The Beatles", got[0].MatchedField.Value)

		bySet, _, err := items.SearchForUser(ctx, alice.ID, "75192", 50)
		require.NoError(t, err)
		require.Equal(t, []string{"Millennium Falcon"}, names(bySet))
		require.Equal(t, "Set number", bySet[0].MatchedField.Label)
	})

	t.Run("LIKE wildcards in the query are literal", func(t *testing.T) {
		got, _, err := items.SearchForUser(ctx, alice.ID, "%", 50)
		require.NoError(t, err)
		require.Equal(t, []string{"100% Brick_Box"}, names(got))

		under, _, err := items.SearchForUser(ctx, alice.ID, "_", 50)
		require.NoError(t, err)
		require.Equal(t, []string{"100% Brick_Box"}, names(under))
	})

	t.Run("empty query returns nothing", func(t *testing.T) {
		got, more, err := items.SearchForUser(ctx, alice.ID, "", 50)
		require.NoError(t, err)
		require.False(t, more)
		require.Empty(t, got)
	})

	t.Run("limit caps results and reports there are more", func(t *testing.T) {
		got, more, err := items.SearchForUser(ctx, alice.ID, "falcon", 1)
		require.NoError(t, err)
		require.True(t, more)
		require.Equal(t, []string{"Falcon Mini Build"}, names(got))
	})
}
