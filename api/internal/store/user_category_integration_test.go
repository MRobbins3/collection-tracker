//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/category"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

type userCategoryFixture struct {
	cats       *store.Categories
	colls      *store.Collections
	items      *store.Items
	overview   *store.Overview
	catalog    *store.Catalog
	mod        *store.Moderation
	users      *store.Users
	alice, bob store.User
	admin      store.User
}

func newUserCategoryFixture(t *testing.T) (*userCategoryFixture, context.Context) {
	t.Helper()
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	users := store.NewUsers(pool)
	alice, err := users.UpsertByGoogleSub(ctx, "alice-sub", "alice@example.com", "Alice")
	require.NoError(t, err)
	bob, err := users.UpsertByGoogleSub(ctx, "bob-sub", "bob@example.com", "Bob")
	require.NoError(t, err)
	admin, err := users.UpsertByGoogleSub(ctx, "olga-sub", "olga@example.com", "Olga")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE users SET role = 'admin' WHERE id = $1`, admin.ID)
	require.NoError(t, err)
	return &userCategoryFixture{
		cats: store.NewCategories(pool), colls: store.NewCollections(pool), items: store.NewItems(pool),
		overview: store.NewOverview(pool), catalog: store.NewCatalog(pool), mod: store.NewModeration(pool),
		users: users, alice: alice, bob: bob, admin: admin,
	}, ctx
}

// sneakerInput composes a schema from the real seeded field library, the
// way the handler does.
func (f *userCategoryFixture) sneakerInput(t *testing.T, ctx context.Context, name string, brandOptions ...string) store.CategoryInput {
	t.Helper()
	srcs, err := f.cats.ApprovedSources(ctx)
	require.NoError(t, err)
	composed, err := category.Compose(category.Library(srcs), nil, []category.Choice{
		{ID: "string:theme", Options: brandOptions},
		{ID: "integer:year"},
	}, "")
	require.NoError(t, err)
	return store.CategoryInput{Name: name, Schema: composed.Schema}
}

func slugsOf(cats []store.Category) []string {
	out := make([]string, 0, len(cats))
	for _, c := range cats {
		out = append(out, c.Slug)
	}
	return out
}

func TestUserCategoryVisibility(t *testing.T) {
	f, ctx := newUserCategoryFixture(t)

	sneakers, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "  Sneakers ", "Nike", "Adidas"))
	require.NoError(t, err)
	require.Equal(t, "Sneakers", sneakers.Name, "the name is cleaned")
	require.Equal(t, store.CategoryPrivate, sneakers.Status)
	require.True(t, sneakers.Mine)
	require.Regexp(t, `^sneakers-[0-9a-f]{6}$`, sneakers.Slug)

	t.Run("the creator sees it in lists, search, and by slug", func(t *testing.T) {
		all, err := f.cats.List(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Contains(t, slugsOf(all), sneakers.Slug)
		require.Len(t, all, 9)
		found, err := f.cats.Search(ctx, f.alice.ID, "sneak")
		require.NoError(t, err)
		require.Equal(t, []string{sneakers.Slug}, slugsOf(found))
		got, err := f.cats.GetBySlug(ctx, f.alice.ID, sneakers.Slug)
		require.NoError(t, err)
		require.True(t, got.Mine)
		mine, err := f.cats.ListMine(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, []string{sneakers.Slug}, slugsOf(mine))
	})

	t.Run("nobody else sees it anywhere", func(t *testing.T) {
		for _, viewer := range []string{"", f.bob.ID, f.admin.ID, "not-a-uuid"} {
			all, err := f.cats.List(ctx, viewer)
			require.NoError(t, err)
			require.NotContains(t, slugsOf(all), sneakers.Slug)
			found, err := f.cats.Search(ctx, viewer, "sneak")
			require.NoError(t, err)
			require.Empty(t, found)
			_, err = f.cats.GetBySlug(ctx, viewer, sneakers.Slug)
			require.ErrorIs(t, err, store.ErrNotFound)
			_, err = f.catalog.List(ctx, store.CatalogQuery{CategorySlug: sneakers.Slug, ViewerID: viewer, Limit: 10})
			require.ErrorIs(t, err, store.ErrNotFound)
		}
		_, err := f.colls.Create(ctx, f.bob.ID, sneakers.Slug, "Bob's Shoes")
		require.ErrorIs(t, err, store.ErrCategoryNotFound)
		require.ErrorIs(t, f.overview.Pin(ctx, f.bob.ID, sneakers.Slug), store.ErrCategoryNotFound)
		bobCats, err := f.overview.Categories(ctx, f.bob.ID)
		require.NoError(t, err)
		require.Empty(t, bobCats)
		mine, err := f.cats.ListMine(ctx, f.bob.ID)
		require.NoError(t, err)
		require.Empty(t, mine)
	})

	t.Run("the creator uses it at once: collection, item, pin, empty catalog", func(t *testing.T) {
		coll, err := f.colls.Create(ctx, f.alice.ID, sneakers.Slug, "My Shoes")
		require.NoError(t, err)
		require.Equal(t, "Sneakers", coll.CategoryName)
		_, err = f.items.Create(ctx, coll.ID, f.alice.ID, store.ItemInput{
			Name: "Air Max 90", Quantity: 1, Attributes: json.RawMessage(`{"theme":"Nike","year":1990}`),
		})
		require.NoError(t, err)
		require.NoError(t, f.overview.Pin(ctx, f.alice.ID, sneakers.Slug))
		ov, err := f.overview.Categories(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Len(t, ov, 1)
		require.Equal(t, "Sneakers", ov[0].Name)
		page, err := f.catalog.List(ctx, store.CatalogQuery{CategorySlug: sneakers.Slug, ViewerID: f.alice.ID, Limit: 10})
		require.NoError(t, err)
		require.Empty(t, page.Entries)
	})

	t.Run("catalog suggestions need an approved category", func(t *testing.T) {
		_, err := f.mod.Submit(ctx, f.alice.ID, store.SubmissionInput{
			CategorySlug: sneakers.Slug, Name: "Air Jordan 1", Attributes: json.RawMessage(`{}`),
		})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("GetByID skips the check, for callers that authorized another way", func(t *testing.T) {
		got, err := f.cats.GetByID(ctx, sneakers.ID)
		require.NoError(t, err)
		require.Equal(t, sneakers.Slug, got.Slug)
		require.False(t, got.Mine)
	})

	t.Run("the field library only reads approved categories", func(t *testing.T) {
		srcs, err := f.cats.ApprovedSources(ctx)
		require.NoError(t, err)
		for _, s := range srcs {
			require.NotEqual(t, sneakers.Slug, s.Slug)
		}
		require.Len(t, srcs, 8)
	})
}

func TestUserCategoryNamesAndEdits(t *testing.T) {
	f, ctx := newUserCategoryFixture(t)

	t.Run("a name matching a category you can see is refused with that category", func(t *testing.T) {
		_, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "lego   SETS"))
		var exists *store.CategoryExistsError
		require.True(t, errors.As(err, &exists), "got %v", err)
		require.Equal(t, "lego-sets", exists.Existing.Slug)

		mine, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "Sneakers"))
		require.NoError(t, err)
		_, err = f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "SNEAKERS"))
		require.True(t, errors.As(err, &exists))
		require.Equal(t, mine.Slug, exists.Existing.Slug)

		_, err = f.cats.Create(ctx, f.bob.ID, f.sneakerInput(t, ctx, "sneakers"))
		require.NoError(t, err, "another user's private category doesn't block (or reveal) the name")
	})

	sneakers, err := f.cats.GetBySlug(ctx, f.alice.ID, mustMineSlug(t, ctx, f, f.alice.ID, "Sneakers"))
	require.NoError(t, err)
	coll, err := f.colls.Create(ctx, f.alice.ID, sneakers.Slug, "Shoes")
	require.NoError(t, err)
	_, err = f.items.Create(ctx, coll.ID, f.alice.ID, store.ItemInput{
		Name: "Samba", Quantity: 1, Attributes: json.RawMessage(`{"theme":"Adidas"}`),
	})
	require.NoError(t, err)

	t.Run("edits keep options and fields the items use", func(t *testing.T) {
		_, err := f.cats.Update(ctx, f.alice.ID, sneakers.Slug, f.sneakerInput(t, ctx, "Sneakers", "Nike"))
		var inUse *store.SchemaInUseError
		require.True(t, errors.As(err, &inUse), "turning free text into a list that leaves out Adidas: %v", err)
		require.Contains(t, inUse.Message, "Adidas")

		srcs, err := f.cats.ApprovedSources(ctx)
		require.NoError(t, err)
		onlyYear, err := category.Compose(category.Library(srcs), nil, []category.Choice{{ID: "integer:year"}}, "")
		require.NoError(t, err)
		_, err = f.cats.Update(ctx, f.alice.ID, sneakers.Slug, store.CategoryInput{Name: "Sneakers", Schema: onlyYear.Schema})
		require.True(t, errors.As(err, &inUse), "removing a field an item uses")

		got, err := f.cats.Update(ctx, f.alice.ID, sneakers.Slug, f.sneakerInput(t, ctx, "Trainers", "Nike", "Adidas"))
		require.NoError(t, err)
		require.Equal(t, "Trainers", got.Name)
		require.Equal(t, sneakers.Slug, got.Slug, "the slug never changes")
	})

	t.Run("a category whose name was later approved for someone else stays editable", func(t *testing.T) {
		alicesMugs, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "Mugs"))
		require.NoError(t, err)
		bobsMugs, err := f.cats.Create(ctx, f.bob.ID, f.sneakerInput(t, ctx, "mugs"))
		require.NoError(t, err)
		sub, err := f.mod.SubmitCategory(ctx, f.bob.ID, bobsMugs.Slug, nil)
		require.NoError(t, err)
		_, err = f.mod.Approve(ctx, f.admin.ID, sub.ID, nil)
		require.NoError(t, err)

		got, err := f.cats.Update(ctx, f.alice.ID, alicesMugs.Slug, f.sneakerInput(t, ctx, "Mugs", "Ceramic"))
		require.NoError(t, err, "keeping its own name, the fields can still change")
		require.Contains(t, string(got.AttributeSchema), "Ceramic")

		_, err = f.cats.Update(ctx, f.alice.ID, sneakers.Slug, f.sneakerInput(t, ctx, "MUGS", "Nike", "Adidas"))
		var exists *store.CategoryExistsError
		require.True(t, errors.As(err, &exists), "renaming onto a public name is refused")
		require.Equal(t, bobsMugs.Slug, exists.Existing.Slug)
	})

	t.Run("only the creator can edit", func(t *testing.T) {
		_, err := f.cats.Update(ctx, f.bob.ID, sneakers.Slug, f.sneakerInput(t, ctx, "Mine Now"))
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = f.cats.Update(ctx, f.alice.ID, "lego-sets", f.sneakerInput(t, ctx, "Lego"))
		require.ErrorIs(t, err, store.ErrNotFound, "system categories have no creator")
	})

	t.Run("submitted categories are locked", func(t *testing.T) {
		_, err := f.mod.SubmitCategory(ctx, f.alice.ID, sneakers.Slug, nil)
		require.NoError(t, err)
		_, err = f.cats.Update(ctx, f.alice.ID, sneakers.Slug, f.sneakerInput(t, ctx, "Trainers", "Nike", "Adidas"))
		require.ErrorIs(t, err, store.ErrCategoryLocked)
		_, err = f.mod.SubmitCategory(ctx, f.alice.ID, sneakers.Slug, nil)
		require.ErrorIs(t, err, store.ErrCategoryLocked, "already waiting")
		_, err = f.mod.SubmitCategory(ctx, f.bob.ID, sneakers.Slug, nil)
		require.ErrorIs(t, err, store.ErrNotFound)
	})
}

func mustMineSlug(t *testing.T, ctx context.Context, f *userCategoryFixture, userID, name string) string {
	t.Helper()
	mine, err := f.cats.ListMine(ctx, userID)
	require.NoError(t, err)
	for _, c := range mine {
		if c.Name == name {
			return c.Slug
		}
	}
	t.Fatalf("no category %q", name)
	return ""
}

func TestUserCategoryModeration(t *testing.T) {
	f, ctx := newUserCategoryFixture(t)
	sneakers, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "Sneakers", "Nike"))
	require.NoError(t, err)

	note := "  Lots of people collect these. "
	sub, err := f.mod.SubmitCategory(ctx, f.alice.ID, sneakers.Slug, &note)
	require.NoError(t, err)
	require.Equal(t, store.KindCategory, sub.Kind)
	require.Equal(t, store.StatusPending, sub.Status)
	require.Equal(t, "Sneakers", sub.Name)
	require.Nil(t, sub.EntryID)

	t.Run("it joins the queue with a schema preview, still private", func(t *testing.T) {
		page, err := f.mod.Queue(ctx, store.QueueQuery{Kind: store.KindCategory, Limit: 10})
		require.NoError(t, err)
		require.Len(t, page.Requests, 1)
		require.Equal(t, "Sneakers", page.Requests[0].Name)
		require.Equal(t, "Lots of people collect these.", *page.Requests[0].Note)

		req, err := f.mod.Get(ctx, sub.ID)
		require.NoError(t, err)
		require.Nil(t, req.Entry)
		require.NotNil(t, req.Category)
		require.Equal(t, store.CategoryPending, req.Category.Status)
		require.Contains(t, string(req.Category.AttributeSchema), "Nike")

		_, err = f.cats.GetBySlug(ctx, f.bob.ID, sneakers.Slug)
		require.ErrorIs(t, err, store.ErrNotFound, "pending is still private")
		_, err = f.mod.Merge(ctx, f.admin.ID, sub.ID, "00000000-0000-0000-0000-000000000000", nil)
		require.ErrorIs(t, err, store.ErrWrongKind)
		name := "Shoes"
		_, err = f.mod.Approve(ctx, f.admin.ID, sub.ID, &store.EntryFields{Name: &name})
		require.ErrorIs(t, err, store.ErrWrongKind, "categories are approved as built")
	})

	t.Run("rejecting keeps it private with the reason shown to the creator", func(t *testing.T) {
		got, err := f.mod.Reject(ctx, f.admin.ID, sub.ID, "Please add a size field.")
		require.NoError(t, err)
		require.Equal(t, store.StatusRejected, got.Status)
		cat, err := f.cats.GetBySlug(ctx, f.alice.ID, sneakers.Slug)
		require.NoError(t, err)
		require.Equal(t, store.CategoryRejected, cat.Status)
		require.Equal(t, "Please add a size field.", *cat.ReviewReason)
		_, err = f.cats.GetBySlug(ctx, f.bob.ID, sneakers.Slug)
		require.ErrorIs(t, err, store.ErrNotFound)
		mine, err := f.mod.ListMine(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, "Please add a size field.", *mine[0].Reason)
	})

	t.Run("resubmitting and approving puts it on the public list", func(t *testing.T) {
		again, err := f.mod.SubmitCategory(ctx, f.alice.ID, sneakers.Slug, nil)
		require.NoError(t, err)
		got, err := f.mod.Approve(ctx, f.admin.ID, again.ID, nil)
		require.NoError(t, err)
		require.Equal(t, store.StatusApproved, got.Status)
		require.Equal(t, store.CategoryApproved, got.Category.Status)

		for _, viewer := range []string{"", f.bob.ID} {
			all, err := f.cats.List(ctx, viewer)
			require.NoError(t, err)
			require.Contains(t, slugsOf(all), sneakers.Slug)
		}
		_, err = f.colls.Create(ctx, f.bob.ID, sneakers.Slug, "Bob's Shoes")
		require.NoError(t, err)
		srcs, err := f.cats.ApprovedSources(ctx)
		require.NoError(t, err)
		require.Len(t, srcs, 9, "its fields join the library")

		_, err = f.cats.Update(ctx, f.alice.ID, sneakers.Slug, f.sneakerInput(t, ctx, "Sneakers", "Nike", "Puma"))
		require.ErrorIs(t, err, store.ErrCategoryLocked, "approved categories change through review, not edits")
		_, err = f.cats.Create(ctx, f.bob.ID, f.sneakerInput(t, ctx, "sneakers"))
		var exists *store.CategoryExistsError
		require.True(t, errors.As(err, &exists), "now everyone can see it, so nobody can duplicate it")
	})

	t.Run("a second category with an approved name can't be approved", func(t *testing.T) {
		// Bob made his private "Mugs" before Alice's was approved.
		bobMugs, err := f.cats.Create(ctx, f.bob.ID, f.sneakerInput(t, ctx, "Mugs"))
		require.NoError(t, err)
		aliceMugs, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "mugs"))
		require.NoError(t, err)
		a, err := f.mod.SubmitCategory(ctx, f.alice.ID, aliceMugs.Slug, nil)
		require.NoError(t, err)
		b, err := f.mod.SubmitCategory(ctx, f.bob.ID, bobMugs.Slug, nil)
		require.NoError(t, err)
		_, err = f.mod.Approve(ctx, f.admin.ID, a.ID, nil)
		require.NoError(t, err)
		_, err = f.mod.Approve(ctx, f.admin.ID, b.ID, nil)
		require.ErrorIs(t, err, store.ErrDuplicateCategory)
		_, err = f.mod.Reject(ctx, f.admin.ID, b.ID, "Mugs is already on the list.")
		require.NoError(t, err)
	})
}

func TestDeletingAUserRemovesTheirUnapprovedCategories(t *testing.T) {
	f, ctx := newUserCategoryFixture(t)
	private, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "Sneakers"))
	require.NoError(t, err)
	_, err = f.colls.Create(ctx, f.alice.ID, private.Slug, "Shoes")
	require.NoError(t, err)
	require.NoError(t, f.overview.Pin(ctx, f.alice.ID, private.Slug))
	public, err := f.cats.Create(ctx, f.alice.ID, f.sneakerInput(t, ctx, "Mugs"))
	require.NoError(t, err)
	sub, err := f.mod.SubmitCategory(ctx, f.alice.ID, public.Slug, nil)
	require.NoError(t, err)
	_, err = f.mod.Approve(ctx, f.admin.ID, sub.ID, nil)
	require.NoError(t, err)

	require.NoError(t, f.users.Delete(ctx, f.alice.ID))

	_, err = f.cats.GetByID(ctx, private.ID)
	require.ErrorIs(t, err, store.ErrNotFound)
	kept, err := f.cats.GetBySlug(ctx, f.bob.ID, public.Slug)
	require.NoError(t, err, "approved categories stay, unattributed")
	require.Equal(t, "Mugs", kept.Name)
}
