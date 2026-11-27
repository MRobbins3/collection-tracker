package server_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/privacy"
	"github.com/MRobbins3/collection-tracker/api/internal/server"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// fakeCompare records the last query and answers with canned data. Its
// one known category is "lego-sets"; anything else is ErrNotFound.
type fakeCompare struct {
	catsFor [2]string
	last    *store.CompareQuery
}

func (f *fakeCompare) Categories(_ context.Context, viewerID, otherID string) ([]store.CompareCategory, error) {
	f.catsFor = [2]string{viewerID, otherID}
	return []store.CompareCategory{{Slug: "lego-sets", Name: "Lego Sets", Public: true, MineItems: 1, TheirItems: 2}}, nil
}

func (f *fakeCompare) Compare(_ context.Context, q store.CompareQuery) (store.CompareResult, error) {
	f.last = &q
	if q.ViewerID == q.OtherID {
		return store.CompareResult{}, store.ErrCompareSelf
	}
	if q.CategorySlug != "lego-sets" {
		return store.CompareResult{}, store.ErrNotFound
	}
	return store.CompareResult{
		Category: store.CompareCategoryInfo{Slug: "lego-sets", Name: "Lego Sets", Public: true, CatalogEntries: 8},
		Counts:   store.CompareCounts{Both: 1, MineOnly: 2, TheirsOnly: 3, Neither: 4},
		Bucket:   q.Bucket,
		Items:    []store.CompareRow{{Key: "name:x", Name: "X", Mine: &store.CompareSide{Quantity: 1, Items: 1}}},
		HasMore:  true,
	}, nil
}

type compareResp struct {
	Other      map[string]any          `json:"other"`
	Categories []store.CompareCategory `json:"categories"`
	Category   *struct {
		Slug           string `json:"slug"`
		CatalogEntries int    `json:"catalog_entries"`
	} `json:"category"`
	Counts  *store.CompareCounts `json:"counts"`
	Bucket  string               `json:"bucket"`
	Items   []store.CompareRow   `json:"items"`
	HasMore bool                 `json:"has_more"`
}

func compareFixture(t *testing.T, viewerID string) (*fakeCompare, http.Handler) {
	t.Helper()
	f := newProfileFixture(t)
	// Liam is Noah's friend; Noah shares with friends only.
	f.rels = fakeRelationships{viewer: "u-liam", owner: "u-noah", rel: privacy.Friend}
	fc := &fakeCompare{}
	deps := f.deps()
	deps.Compare = fc
	if viewerID == "" {
		deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		deps.DBPinger = fakePinger{}
		deps.Sessions = auth.NewManager([]byte("01234567890123456789012345678901"), false)
		return fc, server.NewRouter(deps)
	}
	return fc, newAuthenticatedRouterWithDeps(t, deps, viewerID)
}

func TestCompareRequiresSignIn(t *testing.T) {
	_, h := compareFixture(t, "")
	rr := doJSON(t, h, http.MethodGet, "/users/liam/compare?category=lego-sets", nil)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestCompareReturnsCountsAndOnePage(t *testing.T) {
	fc, h := compareFixture(t, "u-liam")
	rr := doJSON(t, h, http.MethodGet, "/users/Noah/compare?category=lego-sets&bucket=theirs_only&limit=10&offset=20", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got := decode[compareResp](t, rr)
	require.Equal(t, "noah", got.Other["handle"])
	require.NotContains(t, got.Other, "email")
	require.Nil(t, got.Categories, "a later page skips the picker")
	require.Equal(t, [2]string{}, fc.catsFor)
	require.NotNil(t, got.Category)
	require.Equal(t, 8, got.Category.CatalogEntries)
	require.Equal(t, store.CompareCounts{Both: 1, MineOnly: 2, TheirsOnly: 3, Neither: 4}, *got.Counts)
	require.Equal(t, "theirs_only", got.Bucket)
	require.Len(t, got.Items, 1)
	require.True(t, got.HasMore)
	require.Equal(t, store.CompareQuery{
		ViewerID: "u-liam", OtherID: "u-noah", CategorySlug: "lego-sets",
		Bucket: store.BucketTheirsOnly, Limit: 10, Offset: 20,
	}, *fc.last)
}

func TestCompareDefaults(t *testing.T) {
	fc, h := compareFixture(t, "u-liam")
	rr := doJSON(t, h, http.MethodGet, "/users/noah/compare?category=lego-sets", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, store.BucketBoth, fc.last.Bucket)
	require.Equal(t, 50, fc.last.Limit)
	require.Equal(t, 0, fc.last.Offset)
	got := decode[compareResp](t, rr)
	require.Len(t, got.Categories, 1, "the first page has the picker")
	require.Equal(t, [2]string{"u-liam", "u-noah"}, fc.catsFor)
}

func TestCompareWithoutCategoryListsCategoriesOnly(t *testing.T) {
	fc, h := compareFixture(t, "u-liam")
	rr := doJSON(t, h, http.MethodGet, "/users/noah/compare", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got := decode[compareResp](t, rr)
	require.Len(t, got.Categories, 1)
	require.Nil(t, got.Category)
	require.Nil(t, got.Counts)
	require.Empty(t, got.Items)
	require.Nil(t, fc.last, "no comparison is run")
}

// Comparing reads the other person's collections, so it follows the
// privacy rule exactly like their profile: hidden is 404, never 403.
func TestCompareFollowsThePrivacyRule(t *testing.T) {
	cases := []struct {
		name, viewer, path string
	}{
		{"stranger and Friends only", "u-olivia", "/users/noah/compare?category=lego-sets"},
		{"Private profile", "u-liam", "/users/emma/compare?category=lego-sets"},
		{"not a follower of Followers & friends", "u-liam", "/users/amelia/compare?category=lego-sets"},
		{"unknown handle", "u-liam", "/users/nobody/compare?category=lego-sets"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fc, h := compareFixture(t, c.viewer)
			rr := doJSON(t, h, http.MethodGet, c.path, nil)
			require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
			require.Nil(t, fc.last)
		})
	}

	t.Run("a Public profile compares with anyone signed in", func(t *testing.T) {
		_, h := compareFixture(t, "u-noah")
		rr := doJSON(t, h, http.MethodGet, "/users/liam/compare?category=lego-sets", nil)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

func TestCompareRejectsBadRequests(t *testing.T) {
	t.Run("yourself", func(t *testing.T) {
		fc, h := compareFixture(t, "u-liam")
		rr := doJSON(t, h, http.MethodGet, "/users/liam/compare?category=lego-sets", nil)
		require.Equal(t, http.StatusBadRequest, rr.Code)
		require.Nil(t, fc.last)
	})
	for _, q := range []string{
		"category=lego-sets&bucket=everything",
		"category=lego-sets&limit=0",
		"category=lego-sets&limit=101",
		"category=lego-sets&offset=-1",
		"category=lego-sets&offset=x",
	} {
		t.Run(q, func(t *testing.T) {
			_, h := compareFixture(t, "u-liam")
			rr := doJSON(t, h, http.MethodGet, "/users/noah/compare?"+q, nil)
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
		})
	}
	t.Run("unknown or hidden category", func(t *testing.T) {
		_, h := compareFixture(t, "u-liam")
		rr := doJSON(t, h, http.MethodGet, "/users/noah/compare?category=spoons-cccccc", nil)
		require.Equal(t, http.StatusNotFound, rr.Code)
		got := decode[map[string]string](t, rr)
		require.Equal(t, "category_not_found", got["code"])
	})
}
