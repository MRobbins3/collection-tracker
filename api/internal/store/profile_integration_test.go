//go:build integration

package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func TestSignUpAssignsAUniqueHandle(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users := store.NewUsers(pool)

	a, err := users.UpsertByGoogleSub(ctx, "g-1", "liam@example.com", "Liam Johnson")
	require.NoError(t, err)
	require.Equal(t, "liam_johnson", a.Handle)
	require.NotEmpty(t, a.AvatarColor)

	b, err := users.UpsertByGoogleSub(ctx, "g-2", "other@example.com", "  liam   JOHNSON ")
	require.NoError(t, err)
	require.Equal(t, "liam_johnson2", b.Handle, "collisions get a numeric suffix")
	require.Equal(t, "liam JOHNSON", b.DisplayName, "display names are whitespace-normalized")

	c, err := users.UpsertByGoogleSub(ctx, "g-3", "boss@example.com", "Admin")
	require.NoError(t, err)
	require.Equal(t, "admin2", c.Handle, "reserved words are never assigned")

	p, err := users.GetProfile(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, privacy.DefaultVisibility, p.CollectionVisibility)
	require.True(t, p.AcceptFriendRequests)
	require.True(t, p.AcceptFollowRequests)
	require.Empty(t, p.Bio)
}

func TestSignInKeepsUserEditedNameAndHandle(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users := store.NewUsers(pool)

	u, err := users.UpsertByGoogleSub(ctx, "g-1", "a@example.com", "Alice")
	require.NoError(t, err)
	_, err = users.UpdateProfile(ctx, u.ID, store.ProfilePatch{
		DisplayName: strp("Ally"), Handle: strp("ally"),
	})
	require.NoError(t, err)

	again, err := users.UpsertByGoogleSub(ctx, "g-1", "alice@new.example.com", "Alice From Google")
	require.NoError(t, err)
	require.Equal(t, u.ID, again.ID)
	require.Equal(t, "alice@new.example.com", again.Email, "email still follows Google")
	require.Equal(t, "Ally", again.DisplayName, "a name the user chose isn't overwritten at sign-in")
	require.Equal(t, "ally", again.Handle)
}

func TestUpdateProfile(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users := store.NewUsers(pool)

	liam, err := users.UpsertByGoogleSub(ctx, "g-liam", "l@example.com", "Liam")
	require.NoError(t, err)
	emma, err := users.UpsertByGoogleSub(ctx, "g-emma", "e@example.com", "Emma")
	require.NoError(t, err)

	t.Run("partial update changes only the given fields", func(t *testing.T) {
		vis := privacy.Public
		p, err := users.UpdateProfile(ctx, liam.ID, store.ProfilePatch{
			Bio:                  strp("Lego and vinyl."),
			CollectionVisibility: &vis,
			AcceptFollowRequests: boolp(false),
		})
		require.NoError(t, err)
		require.Equal(t, "Lego and vinyl.", p.Bio)
		require.Equal(t, privacy.Public, p.CollectionVisibility)
		require.False(t, p.AcceptFollowRequests)
		require.True(t, p.AcceptFriendRequests)
		require.Equal(t, "Liam", p.DisplayName)
		require.Equal(t, "liam", p.Handle)
	})

	t.Run("handles are normalized and unique case-insensitively", func(t *testing.T) {
		_, err := users.UpdateProfile(ctx, emma.ID, store.ProfilePatch{Handle: strp("LIAM")})
		require.ErrorIs(t, err, store.ErrHandleTaken)
		_, err = users.UpdateProfile(ctx, emma.ID, store.ProfilePatch{Handle: strp("@Liam")})
		require.ErrorIs(t, err, store.ErrHandleTaken)

		p, err := users.UpdateProfile(ctx, emma.ID, store.ProfilePatch{Handle: strp("  Emma_G ")})
		require.NoError(t, err)
		require.Equal(t, "emma_g", p.Handle)
	})

	t.Run("reserved and malformed handles are refused", func(t *testing.T) {
		for _, h := range []string{"admin", "Settings", "me", "x", "has space", "9lives"} {
			_, err := users.UpdateProfile(ctx, emma.ID, store.ProfilePatch{Handle: strp(h)})
			require.ErrorIs(t, err, store.ErrInvalidHandle, h)
		}
	})

	t.Run("keeping your own handle is not a conflict", func(t *testing.T) {
		_, err := users.UpdateProfile(ctx, emma.ID, store.ProfilePatch{Handle: strp("EMMA_G")})
		require.NoError(t, err)
	})

	t.Run("unknown user", func(t *testing.T) {
		_, err := users.UpdateProfile(ctx, "00000000-0000-0000-0000-000000000000",
			store.ProfilePatch{Bio: strp("x")})
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}

func TestHandleAvailabilityAndLookup(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users := store.NewUsers(pool)

	liam, err := users.UpsertByGoogleSub(ctx, "g-liam", "l@example.com", "Liam")
	require.NoError(t, err)
	emma, err := users.UpsertByGoogleSub(ctx, "g-emma", "e@example.com", "Emma")
	require.NoError(t, err)

	cases := []struct {
		handle, asUser string
		want           bool
	}{
		{"liam", emma.ID, false},
		{"LIAM", emma.ID, false},
		{"liam", liam.ID, true}, // your own handle
		{"brand_new", emma.ID, true},
		{"admin", emma.ID, false},
		{"no", emma.ID, false},
	}
	for _, c := range cases {
		got, err := users.HandleAvailable(ctx, c.handle, c.asUser)
		require.NoError(t, err)
		require.Equal(t, c.want, got, c.handle)
	}

	p, err := users.GetProfileByHandle(ctx, "LiAm")
	require.NoError(t, err)
	require.Equal(t, liam.ID, p.UserID)
	_, err = users.GetProfileByHandle(ctx, "nobody_here")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestDeleteUserCascades(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	users := store.NewUsers(pool)
	colls := store.NewCollections(pool)
	items := store.NewItems(pool)

	gone, err := users.UpsertByGoogleSub(ctx, "g-gone", "g@example.com", "Gone Soon")
	require.NoError(t, err)
	stays, err := users.UpsertByGoogleSub(ctx, "g-stays", "s@example.com", "Stays")
	require.NoError(t, err)

	mine, err := colls.Create(ctx, gone.ID, "coins", "Doomed Coins")
	require.NoError(t, err)
	_, err = items.Create(ctx, mine.ID, gone.ID, store.ItemInput{Name: "Penny", Quantity: 1})
	require.NoError(t, err)
	theirs, err := colls.Create(ctx, stays.ID, "coins", "Safe Coins")
	require.NoError(t, err)
	_, err = items.Create(ctx, theirs.ID, stays.ID, store.ItemInput{Name: "Dime", Quantity: 1})
	require.NoError(t, err)
	var entryID string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO catalog_entries (category_id, name, submitted_by)
		SELECT id, 'Submitted Thing', $1 FROM categories WHERE slug = 'coins'
		RETURNING id`, gone.ID).Scan(&entryID))

	require.NoError(t, users.Delete(ctx, gone.ID))

	_, err = users.GetByID(ctx, gone.ID)
	require.ErrorIs(t, err, store.ErrNotFound)
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM collections WHERE user_id = $1`, gone.ID).Scan(&n))
	require.Zero(t, n)
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM items WHERE collection_id = $1`, mine.ID).Scan(&n))
	require.Zero(t, n)
	var submittedBy *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT submitted_by FROM catalog_entries WHERE id = $1`, entryID).Scan(&submittedBy))
	require.Nil(t, submittedBy, "catalog submissions stay, unattributed")

	left, err := items.ListInCollectionForUser(ctx, theirs.ID, stays.ID)
	require.NoError(t, err)
	require.Len(t, left, 1, "other users' data is untouched")

	require.ErrorIs(t, users.Delete(ctx, gone.ID), store.ErrNotFound)
}

func TestRelationshipsUntilFriendsExist(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)
	users := store.NewUsers(pool)
	rels := store.NewRelationships(pool)

	liam, err := users.GetByGoogleSub(ctx, "dev:liam")
	require.NoError(t, err)
	noah, err := users.GetByGoogleSub(ctx, "dev:noah")
	require.NoError(t, err)

	got, err := rels.Between(ctx, noah.ID, liam.ID)
	require.NoError(t, err)
	require.Equal(t, privacy.None, got, "no friends or followers exist yet")
	got, err = rels.Between(ctx, liam.ID, liam.ID)
	require.NoError(t, err)
	require.Equal(t, privacy.Self, got)
	got, err = rels.Between(ctx, "", liam.ID)
	require.NoError(t, err)
	require.Equal(t, privacy.None, got)
}

func TestDevPersonasHaveProfiles(t *testing.T) {
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := seed.Apply(ctx, pool, seed.Options{Env: "test", Dev: true})
	require.NoError(t, err)
	users := store.NewUsers(pool)

	want := map[string]privacy.Visibility{
		"olivia": privacy.Public,
		"liam":   privacy.Public,
		"emma":   privacy.Private,
		"noah":   privacy.FriendsOnly,
		"amelia": privacy.FollowersAndFriends,
	}
	for handle, vis := range want {
		p, err := users.GetProfileByHandle(ctx, handle)
		require.NoError(t, err, handle)
		require.Equal(t, vis, p.CollectionVisibility, handle)
		require.NotEmpty(t, p.Bio, handle)
	}
}
