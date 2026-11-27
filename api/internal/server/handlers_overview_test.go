package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

type pinCall struct {
	userID, slug string
}

type fakeOverview struct {
	mu          sync.Mutex
	categories  []store.OverviewCategory
	page        store.OverviewItemsPage
	settings    store.OverviewSettings
	itemsErr    error
	pinErr      error
	itemCalls   []store.OverviewQuery
	itemUsers   []string
	pins        []pinCall
	unpins      []pinCall
	settingSets []store.OverviewSettings
}

func newFakeOverview() *fakeOverview {
	return &fakeOverview{
		page:     store.OverviewItemsPage{Items: []store.OverviewItem{}},
		settings: store.OverviewSettings{ShowEmptyCategories: true},
	}
}

func (f *fakeOverview) Categories(_ context.Context, _ string) ([]store.OverviewCategory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.categories == nil {
		return []store.OverviewCategory{}, nil
	}
	return f.categories, nil
}

func (f *fakeOverview) Items(_ context.Context, userID string, q store.OverviewQuery) (store.OverviewItemsPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.itemCalls = append(f.itemCalls, q)
	f.itemUsers = append(f.itemUsers, userID)
	if f.itemsErr != nil {
		return store.OverviewItemsPage{}, f.itemsErr
	}
	return f.page, nil
}

func (f *fakeOverview) Pin(_ context.Context, userID, slug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pins = append(f.pins, pinCall{userID, slug})
	return f.pinErr
}

func (f *fakeOverview) Unpin(_ context.Context, userID, slug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unpins = append(f.unpins, pinCall{userID, slug})
	return nil
}

func (f *fakeOverview) Settings(_ context.Context, _ string) (store.OverviewSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settings, nil
}

func (f *fakeOverview) UpdateSettings(_ context.Context, _ string, s store.OverviewSettings) (store.OverviewSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settingSets = append(f.settingSets, s)
	f.settings = s
	return s, nil
}

func newOverviewRouter(t *testing.T, ov server.OverviewStore) http.Handler {
	t.Helper()
	return newAuthenticatedRouterWithDeps(t, server.Deps{
		Categories:  &fakeCategories{},
		Collections: newFakeCollections(),
		Items:       newFakeItems(),
		Overview:    ov,
		Users:       &fakeUsers{user: store.User{ID: "user-1", Email: "u@x", DisplayName: "U"}},
	}, "user-1")
}

type overviewPayload struct {
	Categories          []store.OverviewCategory `json:"categories"`
	ShowEmptyCategories bool                     `json:"show_empty_categories"`
	Items               []store.OverviewItem     `json:"items"`
	Total               int                      `json:"total"`
	HasMore             bool                     `json:"has_more"`
}

func TestOverviewRoutesRequireAuth(t *testing.T) {
	r := server.NewRouter(server.Deps{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		DBPinger:    fakePinger{},
		Categories:  &fakeCategories{},
		Collections: newFakeCollections(),
		Items:       newFakeItems(),
		Overview:    newFakeOverview(),
		Users:       &fakeUsers{},
		Sessions:    auth.NewManager([]byte("01234567890123456789012345678901"), false),
		CORSOrigins: []string{"http://localhost:3000"},
	})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/me/overview"},
		{http.MethodPost, "/me/pinned-categories"},
		{http.MethodDelete, "/me/pinned-categories/coins"},
		{http.MethodPatch, "/me/overview-settings"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := doJSON(t, r, tc.method, tc.path, map[string]any{})
			require.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestGetOverviewDefaults(t *testing.T) {
	ov := newFakeOverview()
	ov.categories = []store.OverviewCategory{{Slug: "lego-sets", Name: "Lego Sets", ItemCount: 2, CollectionCount: 1}}
	ov.page = store.OverviewItemsPage{
		Items: []store.OverviewItem{{ID: "i1", Name: "Tree House", CategorySlug: "lego-sets", CategoryName: "Lego Sets"}},
		Total: 2, HasMore: true,
	}
	ov.settings = store.OverviewSettings{ShowEmptyCategories: false}
	r := newOverviewRouter(t, ov)

	rr := doJSON(t, r, http.MethodGet, "/me/overview", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var p overviewPayload
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &p))
	require.Len(t, p.Categories, 1)
	require.Equal(t, "Lego Sets", p.Categories[0].Name)
	require.False(t, p.ShowEmptyCategories)
	require.Len(t, p.Items, 1)
	require.Equal(t, 2, p.Total)
	require.True(t, p.HasMore)

	require.Len(t, ov.itemCalls, 1)
	require.Equal(t, "user-1", ov.itemUsers[0], "reads only the caller's items")
	require.Equal(t, store.OverviewQuery{Sort: store.SortNewest, Limit: 50}, ov.itemCalls[0])
}

func TestGetOverviewPassesSortFilterAndPaging(t *testing.T) {
	ov := newFakeOverview()
	r := newOverviewRouter(t, ov)

	rr := doJSON(t, r, http.MethodGet,
		"/me/overview?sort=name_desc&category=vinyl-records,%20lego-sets,,vinyl-records&category=coins&limit=20&offset=40", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, store.OverviewQuery{
		Sort:          store.SortNameDesc,
		CategorySlugs: []string{"vinyl-records", "lego-sets", "coins"},
		Limit:         20,
		Offset:        40,
	}, ov.itemCalls[0], "slugs are trimmed, de-duplicated, and blanks dropped")
}

func TestGetOverviewLimitZeroSkipsItems(t *testing.T) {
	ov := newFakeOverview()
	ov.categories = []store.OverviewCategory{{Slug: "coins", Name: "Coins", Pinned: true}}
	r := newOverviewRouter(t, ov)

	rr := doJSON(t, r, http.MethodGet, "/me/overview?limit=0", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var p overviewPayload
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &p))
	require.Len(t, p.Categories, 1)
	require.NotNil(t, p.Items)
	require.Empty(t, p.Items)
	require.Empty(t, ov.itemCalls, "categories only: no items query")
}

func TestGetOverviewRejectsBadParams(t *testing.T) {
	ov := newFakeOverview()
	r := newOverviewRouter(t, ov)

	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = "cat-" + itoa(i)
	}
	for _, q := range []string{
		"sort=price",
		"sort=NEWEST",
		"category=Lego%20Sets",
		"category=" + strings.Repeat("a", 101),
		"category=" + strings.Join(tooMany, ","),
		"limit=101",
		"limit=-1",
		"limit=ten",
		"offset=-1",
		"offset=10001",
		"offset=x",
	} {
		t.Run(q, func(t *testing.T) {
			rr := doJSON(t, r, http.MethodGet, "/me/overview?"+q, nil)
			require.Equal(t, http.StatusBadRequest, rr.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			require.NotEmpty(t, body["error"])
		})
	}
	require.Empty(t, ov.itemCalls)
}

func TestGetOverviewSurfaces500(t *testing.T) {
	ov := newFakeOverview()
	ov.itemsErr = errors.New("db down")
	r := newOverviewRouter(t, ov)
	rr := doJSON(t, r, http.MethodGet, "/me/overview", nil)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.NotContains(t, rr.Body.String(), "db down")
}

func TestPinCategory(t *testing.T) {
	ov := newFakeOverview()
	r := newOverviewRouter(t, ov)

	rr := doJSON(t, r, http.MethodPost, "/me/pinned-categories", map[string]any{"category_slug": " coins "})
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, []pinCall{{"user-1", "coins"}}, ov.pins)
}

func TestPinCategoryValidation(t *testing.T) {
	ov := newFakeOverview()
	r := newOverviewRouter(t, ov)

	for name, body := range map[string]any{
		"missing slug":  map[string]any{},
		"blank slug":    map[string]any{"category_slug": "  "},
		"bad slug":      map[string]any{"category_slug": "Not A Slug"},
		"unknown field": map[string]any{"category_slug": "coins", "pinned": true},
	} {
		t.Run(name, func(t *testing.T) {
			rr := doJSON(t, r, http.MethodPost, "/me/pinned-categories", body)
			require.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
	require.Empty(t, ov.pins)

	ov.pinErr = store.ErrCategoryNotFound
	rr := doJSON(t, r, http.MethodPost, "/me/pinned-categories", map[string]any{"category_slug": "no-such"})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "unknown category")
}

func TestUnpinCategory(t *testing.T) {
	ov := newFakeOverview()
	r := newOverviewRouter(t, ov)

	rr := doJSON(t, r, http.MethodDelete, "/me/pinned-categories/coins", nil)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, []pinCall{{"user-1", "coins"}}, ov.unpins)
}

func TestPatchOverviewSettings(t *testing.T) {
	ov := newFakeOverview()
	r := newOverviewRouter(t, ov)

	rr := doJSON(t, r, http.MethodPatch, "/me/overview-settings", map[string]any{"show_empty_categories": false})
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"show_empty_categories": false}`, rr.Body.String())
	require.Equal(t, []store.OverviewSettings{{ShowEmptyCategories: false}}, ov.settingSets)

	for name, body := range map[string]any{
		"missing field": map[string]any{},
		"wrong type":    map[string]any{"show_empty_categories": "no"},
		"unknown field": map[string]any{"show_empty_categories": true, "x": 1},
	} {
		t.Run(name, func(t *testing.T) {
			rr := doJSON(t, r, http.MethodPatch, "/me/overview-settings", body)
			require.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
	require.Len(t, ov.settingSets, 1)
}
