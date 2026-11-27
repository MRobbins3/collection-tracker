//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// moderationFixture is two regular users with Lego collections, an admin,
// and one approved Lego entry, on a fresh seeded database.
type moderationFixture struct {
	pool       *pgxpool.Pool
	mod        *store.Moderation
	cat        *store.Catalog
	items      *store.Items
	alice, bob store.User
	admin      store.User
	aliceLego  store.Collection
	aliceVinyl store.Collection
	falconID   string
}

func newModerationFixture(t *testing.T) (*moderationFixture, context.Context) {
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

	colls := store.NewCollections(pool)
	aliceLego, err := colls.Create(ctx, alice.ID, "lego-sets", "Alice's Lego")
	require.NoError(t, err)
	aliceVinyl, err := colls.Create(ctx, alice.ID, "vinyl-records", "Alice's Records")
	require.NoError(t, err)

	falcon := insertCatalogEntry(t, pool, "lego-sets", "Millennium Falcon", "approved",
		map[string]any{"set_number": "75192", "theme": "Star Wars", "piece_count": 7541})

	return &moderationFixture{
		pool: pool, mod: store.NewModeration(pool), cat: store.NewCatalog(pool), items: store.NewItems(pool),
		alice: alice, bob: bob, admin: admin, aliceLego: aliceLego, aliceVinyl: aliceVinyl, falconID: falcon,
	}, ctx
}

func (f *moderationFixture) submit(t *testing.T, ctx context.Context, userID, name, setNumber string, itemID *string) store.SubmitResult {
	t.Helper()
	res, err := f.mod.Submit(ctx, userID, store.SubmissionInput{
		CategorySlug: "lego-sets", Name: name,
		Attributes:   json.RawMessage(`{"set_number":"` + setNumber + `"}`),
		SourceItemID: itemID,
	})
	require.NoError(t, err)
	return res
}

func (f *moderationFixture) freeFormItem(t *testing.T, ctx context.Context, name string) store.Item {
	t.Helper()
	it, err := f.items.Create(ctx, f.aliceLego.ID, f.alice.ID, store.ItemInput{
		Name: name, Quantity: 1, Attributes: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	return it
}

func (f *moderationFixture) entryStatus(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	var status string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT status FROM catalog_entries WHERE id = $1`, id).Scan(&status))
	return status
}

func (f *moderationFixture) countEntries(t *testing.T, ctx context.Context) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_entries`).Scan(&n))
	return n
}

func TestModerationSubmit(t *testing.T) {
	f, ctx := newModerationFixture(t)

	var towerEntryID string
	t.Run("a submission is a pending, user-submitted entry with a cleaned name and copied identifier", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "  Ultimate   Tower ", "10305", nil)
		require.Equal(t, store.OutcomeSubmitted, res.Outcome)
		require.NotNil(t, res.Submission)
		require.Equal(t, store.StatusPending, res.Submission.Status)
		require.Equal(t, store.KindCatalogEntry, res.Submission.Kind)
		require.Equal(t, "Ultimate Tower", res.Submission.Name)
		require.Equal(t, "10305", *res.Submission.Identifier)
		require.Nil(t, res.Submission.EntryID, "no public entry while pending")

		var source, status string
		var submittedBy *string
		require.NoError(t, f.pool.QueryRow(ctx, `
			SELECT ce.id, ce.source, ce.status, ce.submitted_by
			FROM catalog_entries ce WHERE ce.name = 'Ultimate Tower'`).Scan(&towerEntryID, &source, &status, &submittedBy))
		require.Equal(t, "user_submitted", source)
		require.Equal(t, "pending", status)
		require.Equal(t, f.alice.ID, *submittedBy)
	})

	t.Run("pending submissions are never public", func(t *testing.T) {
		page, err := f.cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "tower", Limit: 50})
		require.NoError(t, err)
		require.Empty(t, page.Entries)
		_, err = f.cat.Get(ctx, towerEntryID)
		require.ErrorIs(t, err, store.ErrNotFound)

		it := f.freeFormItem(t, ctx, "Ultimate Tower")
		suggestions, err := f.cat.Suggest(ctx, it.ID, f.aliceLego.ID, f.alice.ID, 5)
		require.NoError(t, err)
		require.Empty(t, suggestions)
		_, err = f.items.SetCatalogEntry(ctx, it.ID, f.aliceLego.ID, f.alice.ID, &towerEntryID)
		require.ErrorIs(t, err, store.ErrCatalogEntryUnavailable, "items can't link to a pending entry")
	})

	t.Run("resubmitting your own pending entry returns it instead of duplicating", func(t *testing.T) {
		before := f.countEntries(t, ctx)
		res := f.submit(t, ctx, f.alice.ID, "ULTIMATE TOWER", " 10305", nil)
		require.Equal(t, store.OutcomeAlreadySubmitted, res.Outcome)
		require.NotNil(t, res.Submission)
		require.Equal(t, "Ultimate Tower", res.Submission.Name)
		require.Equal(t, before, f.countEntries(t, ctx))
	})

	t.Run("someone else's pending entry is reported without its details", func(t *testing.T) {
		before := f.countEntries(t, ctx)
		res := f.submit(t, ctx, f.bob.ID, "ultimate tower", "10305", nil)
		require.Equal(t, store.OutcomePendingReview, res.Outcome)
		require.Nil(t, res.Submission)
		require.Nil(t, res.Entry)
		require.Equal(t, before, f.countEntries(t, ctx))
	})

	t.Run("an approved match is reused, and the source item linked to it", func(t *testing.T) {
		it := f.freeFormItem(t, ctx, "My Falcon")
		before := f.countEntries(t, ctx)
		res := f.submit(t, ctx, f.alice.ID, " millennium  FALCON", "75192", &it.ID)
		require.Equal(t, store.OutcomeAlreadyInCatalog, res.Outcome)
		require.NotNil(t, res.Entry)
		require.Equal(t, f.falconID, res.Entry.ID)
		require.True(t, res.ItemLinked)
		require.Equal(t, before, f.countEntries(t, ctx))

		got, err := f.items.GetInCollectionForUser(ctx, it.ID, f.aliceLego.ID, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, f.falconID, *got.CatalogEntryID)
		require.Equal(t, "My Falcon", got.Name, "linking never rewrites the owner's name")
	})

	t.Run("a different identifier is a different entry", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "Millennium Falcon", "10179", nil)
		require.Equal(t, store.OutcomeSubmitted, res.Outcome)
	})

	t.Run("the source item must be yours and in the same category", func(t *testing.T) {
		vinylItem, err := f.items.Create(ctx, f.aliceVinyl.ID, f.alice.ID, store.ItemInput{Name: "Abbey Road", Quantity: 1})
		require.NoError(t, err)
		_, err = f.mod.Submit(ctx, f.alice.ID, store.SubmissionInput{
			CategorySlug: "lego-sets", Name: "Something", Attributes: json.RawMessage(`{}`), SourceItemID: &vinylItem.ID,
		})
		require.ErrorIs(t, err, store.ErrNotFound, "other category")

		mine := f.freeFormItem(t, ctx, "Mine")
		_, err = f.mod.Submit(ctx, f.bob.ID, store.SubmissionInput{
			CategorySlug: "lego-sets", Name: "Something", Attributes: json.RawMessage(`{}`), SourceItemID: &mine.ID,
		})
		require.ErrorIs(t, err, store.ErrNotFound, "someone else's item")

		bad := "not-a-uuid"
		_, err = f.mod.Submit(ctx, f.alice.ID, store.SubmissionInput{
			CategorySlug: "lego-sets", Name: "Something", Attributes: json.RawMessage(`{}`), SourceItemID: &bad,
		})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("unknown category is not found", func(t *testing.T) {
		_, err := f.mod.Submit(ctx, f.alice.ID, store.SubmissionInput{CategorySlug: "nope", Name: "X", Attributes: json.RawMessage(`{}`)})
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("a category without an identifier keys on the name alone", func(t *testing.T) {
		res, err := f.mod.Submit(ctx, f.alice.ID, store.SubmissionInput{
			CategorySlug: "plants", Name: "Monstera Deliciosa", Attributes: json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		require.Equal(t, store.OutcomeSubmitted, res.Outcome)
		require.Nil(t, res.Submission.Identifier)
	})

	t.Run("my submissions lists only mine, newest first", func(t *testing.T) {
		mine, err := f.mod.ListMine(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Len(t, mine, 3)
		require.Equal(t, "Monstera Deliciosa", mine[0].Name)
		require.Equal(t, "Ultimate Tower", mine[2].Name)
		require.Equal(t, "Lego Sets", mine[2].CategoryName)

		theirs, err := f.mod.ListMine(ctx, f.bob.ID)
		require.NoError(t, err)
		require.Empty(t, theirs)
	})
}

func TestModerationPendingCap(t *testing.T) {
	f, ctx := newModerationFixture(t)
	for i := 0; i < store.MaxPendingPerUser; i++ {
		f.submit(t, ctx, f.bob.ID, "Set", itoa(i), nil)
	}
	_, err := f.mod.Submit(ctx, f.bob.ID, store.SubmissionInput{
		CategorySlug: "lego-sets", Name: "One Too Many", Attributes: json.RawMessage(`{"set_number":"999"}`),
	})
	require.ErrorIs(t, err, store.ErrSubmissionLimit)
	_, err = f.mod.ProposeCorrection(ctx, f.bob.ID, f.falconID, store.EntryFields{Name: strp("Falcon")}, nil)
	require.ErrorIs(t, err, store.ErrSubmissionLimit, "corrections count toward the same cap")

	// Once one is decided there's room again.
	page, err := f.mod.Queue(ctx, store.QueueQuery{Limit: 1})
	require.NoError(t, err)
	_, err = f.mod.Reject(ctx, f.admin.ID, page.Requests[0].ID, "Duplicate")
	require.NoError(t, err)
	f.submit(t, ctx, f.bob.ID, "Now Fits", "1000", nil)
}

func TestModerationDecisions(t *testing.T) {
	f, ctx := newModerationFixture(t)

	t.Run("approving publishes the entry, records who and when, and links the source item", func(t *testing.T) {
		it := f.freeFormItem(t, ctx, "my tower")
		res := f.submit(t, ctx, f.alice.ID, "Ultimate Tower", "10305", &it.ID)

		got, err := f.mod.Approve(ctx, f.admin.ID, res.Submission.ID, nil)
		require.NoError(t, err)
		require.Equal(t, store.StatusApproved, got.Status)
		require.NotNil(t, got.DecidedBy)
		require.Equal(t, "Olga", got.DecidedBy.DisplayName)
		require.NotNil(t, got.DecidedAt)
		require.Equal(t, "approved", got.Entry.Status)

		var approvedBy *string
		var approvedAt *time.Time
		require.NoError(t, f.pool.QueryRow(ctx,
			`SELECT approved_by, approved_at FROM catalog_entries WHERE id = $1`, got.Entry.ID).Scan(&approvedBy, &approvedAt))
		require.Equal(t, f.admin.ID, *approvedBy)
		require.NotNil(t, approvedAt)

		page, err := f.cat.List(ctx, store.CatalogQuery{CategorySlug: "lego-sets", Q: "tower", Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{"Ultimate Tower"}, entryNames(page.Entries))

		linked, err := f.items.GetInCollectionForUser(ctx, it.ID, f.aliceLego.ID, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, got.Entry.ID, *linked.CatalogEntryID)

		mine, err := f.mod.ListMine(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, store.StatusApproved, mine[0].Status)
		require.Equal(t, got.Entry.ID, *mine[0].EntryID)
	})

	t.Run("deciding twice is refused", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "Twice", "2", nil)
		_, err := f.mod.Reject(ctx, f.admin.ID, res.Submission.ID, "No")
		require.NoError(t, err)
		_, err = f.mod.Approve(ctx, f.admin.ID, res.Submission.ID, nil)
		require.ErrorIs(t, err, store.ErrAlreadyDecided)
		_, err = f.mod.Reject(ctx, f.admin.ID, res.Submission.ID, "Again")
		require.ErrorIs(t, err, store.ErrAlreadyDecided)
	})

	t.Run("two admins deciding at once: exactly one wins", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			res := f.submit(t, ctx, f.alice.ID, "Race", itoa(100+i), nil)
			var wg sync.WaitGroup
			errs := make([]error, 2)
			wg.Add(2)
			go func() { defer wg.Done(); _, errs[0] = f.mod.Approve(ctx, f.admin.ID, res.Submission.ID, nil) }()
			go func() { defer wg.Done(); _, errs[1] = f.mod.Reject(ctx, f.admin.ID, res.Submission.ID, "No") }()
			wg.Wait()
			wins := 0
			for _, err := range errs {
				if err == nil {
					wins++
				} else {
					require.ErrorIs(t, err, store.ErrAlreadyDecided)
				}
			}
			require.Equal(t, 1, wins)
			got, err := f.mod.Get(ctx, res.Submission.ID)
			require.NoError(t, err)
			require.Equal(t, got.Status == store.StatusApproved, got.Entry.Status == "approved",
				"the entry matches the winning decision")
		}
	})

	t.Run("rejecting keeps the entry private, shows the reason, and frees the name", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "Blurry Set", "3", nil)
		got, err := f.mod.Reject(ctx, f.admin.ID, res.Submission.ID, "Please include the set number from the box.")
		require.NoError(t, err)
		require.Equal(t, store.StatusRejected, got.Status)
		require.Equal(t, "rejected", f.entryStatus(t, ctx, got.Entry.ID))
		_, err = f.cat.Get(ctx, got.Entry.ID)
		require.ErrorIs(t, err, store.ErrNotFound)

		mine, err := f.mod.ListMine(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, "Please include the set number from the box.", *mine[0].Reason)
		require.Nil(t, mine[0].EntryID)

		again := f.submit(t, ctx, f.alice.ID, "Blurry Set", "3", nil)
		require.Equal(t, store.OutcomeSubmitted, again.Outcome, "a rejected entry doesn't block a new submission")
	})

	t.Run("edit-then-approve applies the edits and keeps what was submitted", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "hogwarts castel", "71043", nil)
		got, err := f.mod.Approve(ctx, f.admin.ID, res.Submission.ID, &store.EntryFields{
			Name:       strp("Hogwarts Castle"),
			Attributes: json.RawMessage(`{"set_number":"71043","theme":"Harry Potter"}`),
		})
		require.NoError(t, err)
		require.Equal(t, "Hogwarts Castle", got.Entry.Name)
		require.Equal(t, "71043", *got.Entry.Identifier)
		require.JSONEq(t, `{"set_number":"71043","theme":"Harry Potter"}`, string(got.Entry.Attributes))
		require.NotNil(t, got.Previous)
		require.Equal(t, "hogwarts castel", *got.Previous.Name)
		require.NotNil(t, got.Applied)
		require.Equal(t, "Hogwarts Castle", *got.Applied.Name)
	})

	t.Run("an edit that collides with an existing entry is refused and stays pending", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "Falcon Typo", "75192", nil)
		_, err := f.mod.Approve(ctx, f.admin.ID, res.Submission.ID, &store.EntryFields{Name: strp("millennium falcon")})
		require.ErrorIs(t, err, store.ErrDuplicateEntry)
		got, err := f.mod.Get(ctx, res.Submission.ID)
		require.NoError(t, err)
		require.Equal(t, store.StatusPending, got.Status)
		require.Equal(t, "pending", got.Entry.Status)
	})

	t.Run("merging a duplicate points the submitter at the existing entry", func(t *testing.T) {
		it := f.freeFormItem(t, ctx, "falcon (mine)")
		res := f.submit(t, ctx, f.alice.ID, "Falcon UCS", "75192", &it.ID)
		got, err := f.mod.Merge(ctx, f.admin.ID, res.Submission.ID, f.falconID, nil)
		require.NoError(t, err)
		require.Equal(t, store.StatusMerged, got.Status)
		require.NotNil(t, got.MergedInto)
		require.Equal(t, "Millennium Falcon", got.MergedInto.Name)
		require.Equal(t, "rejected", f.entryStatus(t, ctx, got.Entry.ID), "the duplicate never goes public")

		linked, err := f.items.GetInCollectionForUser(ctx, it.ID, f.aliceLego.ID, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, f.falconID, *linked.CatalogEntryID)

		mine, err := f.mod.ListMine(ctx, f.alice.ID)
		require.NoError(t, err)
		require.Equal(t, store.StatusMerged, mine[0].Status)
		require.Equal(t, f.falconID, *mine[0].EntryID)
		require.Equal(t, "Millennium Falcon", *mine[0].MergedIntoName)
	})

	t.Run("merge targets must be approved entries in the same category", func(t *testing.T) {
		res := f.submit(t, ctx, f.alice.ID, "Merge Me", "4", nil)
		other := f.submit(t, ctx, f.bob.ID, "Still Pending", "5", nil)
		var pendingEntry string
		require.NoError(t, f.pool.QueryRow(ctx,
			`SELECT catalog_entry_id FROM moderation_requests WHERE id = $1`, other.Submission.ID).Scan(&pendingEntry))
		abbey := insertCatalogEntry(t, f.pool, "vinyl-records", "Abbey Road", "approved", map[string]any{"artist": "The Beatles"})

		for _, target := range []string{pendingEntry, abbey, "not-a-uuid"} {
			_, err := f.mod.Merge(ctx, f.admin.ID, res.Submission.ID, target, nil)
			require.ErrorIs(t, err, store.ErrMergeTarget, target)
		}
		got, err := f.mod.Get(ctx, res.Submission.ID)
		require.NoError(t, err)
		require.Equal(t, store.StatusPending, got.Status)
	})

	t.Run("unknown requests are not found", func(t *testing.T) {
		for _, id := range []string{"00000000-0000-4000-8000-000000000000", "nope"} {
			_, err := f.mod.Get(ctx, id)
			require.ErrorIs(t, err, store.ErrNotFound)
			_, err = f.mod.Approve(ctx, f.admin.ID, id, nil)
			require.ErrorIs(t, err, store.ErrNotFound)
		}
	})
}

func TestModerationCorrections(t *testing.T) {
	f, ctx := newModerationFixture(t)

	t.Run("a correction stores only the changed fields and leaves the entry alone", func(t *testing.T) {
		sub, err := f.mod.ProposeCorrection(ctx, f.alice.ID, f.falconID, store.EntryFields{
			Name:       strp("Millennium Falcon"), // unchanged
			Attributes: json.RawMessage(`{"set_number":"75192","theme":"Star Wars","piece_count":7541,"year":2017}`),
		}, strp("Year from the box"))
		require.NoError(t, err)
		require.Equal(t, store.KindCatalogCorrection, sub.Kind)
		require.Equal(t, store.StatusPending, sub.Status)
		require.NotNil(t, sub.Proposed)
		require.Nil(t, sub.Proposed.Name)
		require.NotNil(t, sub.Proposed.Attributes)
		require.Equal(t, f.falconID, *sub.EntryID, "corrections point at the public entry")

		e, err := f.cat.Get(ctx, f.falconID)
		require.NoError(t, err)
		require.NotContains(t, string(e.Attributes), "year")
	})

	t.Run("no changes is refused", func(t *testing.T) {
		_, err := f.mod.ProposeCorrection(ctx, f.alice.ID, f.falconID, store.EntryFields{
			Name:       strp(" Millennium  Falcon "),
			Attributes: json.RawMessage(`{"piece_count":7541,"theme":"Star Wars","set_number":"75192"}`),
		}, nil)
		require.ErrorIs(t, err, store.ErrNoChanges)
	})

	t.Run("only approved entries can be corrected", func(t *testing.T) {
		res := f.submit(t, ctx, f.bob.ID, "Pending Thing", "6", nil)
		var pendingEntry string
		require.NoError(t, f.pool.QueryRow(ctx,
			`SELECT catalog_entry_id FROM moderation_requests WHERE id = $1`, res.Submission.ID).Scan(&pendingEntry))
		_, err := f.mod.ProposeCorrection(ctx, f.alice.ID, pendingEntry, store.EntryFields{Name: strp("X")}, nil)
		require.ErrorIs(t, err, store.ErrNotFound)
		_, err = f.mod.ProposeCorrection(ctx, f.alice.ID, "nope", store.EntryFields{Name: strp("X")}, nil)
		require.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("approving applies it and records the previous values", func(t *testing.T) {
		sub, err := f.mod.ProposeCorrection(ctx, f.bob.ID, f.falconID, store.EntryFields{
			Name: strp("Millennium Falcon UCS"), Description: strp("Ultimate Collector Series."),
		}, nil)
		require.NoError(t, err)
		got, err := f.mod.Approve(ctx, f.admin.ID, sub.ID, nil)
		require.NoError(t, err)
		require.Equal(t, store.StatusApproved, got.Status)
		require.Equal(t, "Millennium Falcon", *got.Previous.Name)

		e, err := f.cat.Get(ctx, f.falconID)
		require.NoError(t, err)
		require.Equal(t, "Millennium Falcon UCS", e.Name)
		require.Equal(t, "Ultimate Collector Series.", *e.Description)
		require.Equal(t, "75192", *e.Identifier)
	})

	t.Run("edit-then-approve applies the admin's version, including a new identifier", func(t *testing.T) {
		sub, err := f.mod.ProposeCorrection(ctx, f.alice.ID, f.falconID, store.EntryFields{
			Attributes: json.RawMessage(`{"set_number":"75192 ","theme":"Star Wars","piece_count":7500}`),
		}, nil)
		require.NoError(t, err)
		got, err := f.mod.Approve(ctx, f.admin.ID, sub.ID, &store.EntryFields{
			Attributes: json.RawMessage(`{"set_number":"75192","theme":"Star Wars","piece_count":7541,"year":2017}`),
		})
		require.NoError(t, err)
		require.JSONEq(t, `{"set_number":"75192","theme":"Star Wars","piece_count":7541,"year":2017}`, string(got.Applied.Attributes))
		e, err := f.cat.Get(ctx, f.falconID)
		require.NoError(t, err)
		require.Contains(t, string(e.Attributes), "2017")
	})

	t.Run("rejecting leaves the entry exactly as it was", func(t *testing.T) {
		before, err := f.cat.Get(ctx, f.falconID)
		require.NoError(t, err)
		sub, err := f.mod.ProposeCorrection(ctx, f.alice.ID, f.falconID, store.EntryFields{Name: strp("Falcon")}, nil)
		require.NoError(t, err)
		_, err = f.mod.Reject(ctx, f.admin.ID, sub.ID, "The full name is correct.")
		require.NoError(t, err)
		after, err := f.cat.Get(ctx, f.falconID)
		require.NoError(t, err)
		require.Equal(t, before.Name, after.Name)
		require.Equal(t, before.UpdatedAt, after.UpdatedAt)
	})

	t.Run("a correction can't be merged", func(t *testing.T) {
		sub, err := f.mod.ProposeCorrection(ctx, f.alice.ID, f.falconID, store.EntryFields{Name: strp("Falcon 2")}, nil)
		require.NoError(t, err)
		_, err = f.mod.Merge(ctx, f.admin.ID, sub.ID, f.falconID, nil)
		require.ErrorIs(t, err, store.ErrWrongKind)
	})

	t.Run("a correction that would duplicate another entry is refused on approval", func(t *testing.T) {
		insertCatalogEntry(t, f.pool, "lego-sets", "AT-AT", "approved", map[string]any{"set_number": "75313"})
		sub, err := f.mod.ProposeCorrection(ctx, f.alice.ID, f.falconID, store.EntryFields{
			Name: strp("AT-AT"), Attributes: json.RawMessage(`{"set_number":"75313"}`),
		}, nil)
		require.NoError(t, err)
		_, err = f.mod.Approve(ctx, f.admin.ID, sub.ID, nil)
		require.ErrorIs(t, err, store.ErrDuplicateEntry)
	})
}

func TestModerationQueue(t *testing.T) {
	f, ctx := newModerationFixture(t)

	a := f.submit(t, ctx, f.alice.ID, "Alpha", "1", nil)
	b := f.submit(t, ctx, f.bob.ID, "Bravo", "2", nil)
	plants, err := f.mod.Submit(ctx, f.alice.ID, store.SubmissionInput{CategorySlug: "plants", Name: "Pothos", Attributes: json.RawMessage(`{}`)})
	require.NoError(t, err)
	corr, err := f.mod.ProposeCorrection(ctx, f.bob.ID, f.falconID, store.EntryFields{Name: strp("Falcon")}, strp("Shorter"))
	require.NoError(t, err)
	decided := f.submit(t, ctx, f.alice.ID, "Decided", "9", nil)
	_, err = f.mod.Reject(ctx, f.admin.ID, decided.Submission.ID, "No")
	require.NoError(t, err)

	// Alpha and Bravo share a created_at, so the id breaks the tie.
	_, err = f.pool.Exec(ctx, `UPDATE moderation_requests SET created_at = '2026-01-01T00:00:00Z' WHERE id = ANY($1)`,
		[]string{a.Submission.ID, b.Submission.ID})
	require.NoError(t, err)
	firstTwo := []string{a.Submission.ID, b.Submission.ID}
	if firstTwo[1] < firstTwo[0] {
		firstTwo[0], firstTwo[1] = firstTwo[1], firstTwo[0]
	}

	ids := func(p store.QueuePage) []string {
		out := []string{}
		for _, r := range p.Requests {
			out = append(out, r.ID)
		}
		return out
	}

	t.Run("pending only, oldest first, id breaks ties", func(t *testing.T) {
		page, err := f.mod.Queue(ctx, store.QueueQuery{Limit: 50})
		require.NoError(t, err)
		require.Equal(t, append(firstTwo, plants.Submission.ID, corr.ID), ids(page))
		require.Equal(t, 4, page.Total)
		require.False(t, page.HasMore)
		require.Equal(t, "Shorter", *page.Requests[3].Note)
		require.Equal(t, "Millennium Falcon", page.Requests[3].Name, "a correction shows the entry it changes")
		require.Equal(t, "Bob", page.Requests[3].Submitter.DisplayName)
	})

	t.Run("pages are stable across tied timestamps", func(t *testing.T) {
		p1, err := f.mod.Queue(ctx, store.QueueQuery{Limit: 1})
		require.NoError(t, err)
		require.True(t, p1.HasMore)
		p2, err := f.mod.Queue(ctx, store.QueueQuery{Limit: 1, Offset: 1})
		require.NoError(t, err)
		require.Equal(t, firstTwo, append(ids(p1), ids(p2)...))
	})

	t.Run("filters by kind and category", func(t *testing.T) {
		page, err := f.mod.Queue(ctx, store.QueueQuery{Kind: store.KindCatalogCorrection, Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{corr.ID}, ids(page))

		page, err = f.mod.Queue(ctx, store.QueueQuery{CategorySlug: "plants", Limit: 50})
		require.NoError(t, err)
		require.Equal(t, []string{plants.Submission.ID}, ids(page))

		page, err = f.mod.Queue(ctx, store.QueueQuery{CategorySlug: "lego-sets", Kind: store.KindCatalogEntry, Limit: 50})
		require.NoError(t, err)
		require.Equal(t, firstTwo, ids(page))
	})
}

// Deleting an account removes the user's requests and their unpublished
// submissions; entries already approved stay in the catalog, unattributed.
func TestDeleteUserRemovesUnpublishedSubmissions(t *testing.T) {
	f, ctx := newModerationFixture(t)
	pending := f.submit(t, ctx, f.bob.ID, "Pending", "1", nil)
	approved := f.submit(t, ctx, f.bob.ID, "Approved", "2", nil)
	got, err := f.mod.Approve(ctx, f.admin.ID, approved.Submission.ID, nil)
	require.NoError(t, err)
	pendingDetail, err := f.mod.Get(ctx, pending.Submission.ID)
	require.NoError(t, err)

	require.NoError(t, store.NewUsers(f.pool).Delete(ctx, f.bob.ID))

	var n int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM catalog_entries WHERE id = $1`, pendingDetail.Entry.ID).Scan(&n))
	require.Zero(t, n, "the pending entry is gone")
	_, err = f.cat.Get(ctx, got.Entry.ID)
	require.NoError(t, err, "the approved entry stays")
	_, err = f.mod.Get(ctx, pending.Submission.ID)
	require.True(t, errors.Is(err, store.ErrNotFound))
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
