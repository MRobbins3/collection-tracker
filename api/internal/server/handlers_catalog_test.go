package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// ---- fake -----------------------------------------------------------------

type fakeCatalog struct {
	entries []store.CatalogEntry
	page    store.CatalogPage
	err     error

	listCalls []store.CatalogQuery

	suggestions  []store.CatalogSuggestion
	suggestErr   error
	suggestCalls int

	owned      []store.CatalogOwnership
	ownedCalls []string // "userID|slug"
}

func (f *fakeCatalog) List(_ context.Context, q store.CatalogQuery) (store.CatalogPage, error) {
	f.listCalls = append(f.listCalls, q)
	if f.err != nil {
		return store.CatalogPage{}, f.err
	}
	if q.CategorySlug == "unknown" {
		return store.CatalogPage{}, store.ErrNotFound
	}
	if f.page.Entries != nil {
		return f.page, nil
	}
	entries := f.entries
	if entries == nil {
		entries = []store.CatalogEntry{}
	}
	return store.CatalogPage{Entries: entries, Total: len(entries)}, nil
}

func (f *fakeCatalog) Get(_ context.Context, id string) (store.CatalogEntry, error) {
	for _, e := range f.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return store.CatalogEntry{}, store.ErrNotFound
}

func (f *fakeCatalog) Suggest(_ context.Context, _, _, _ string, _ int) ([]store.CatalogSuggestion, error) {
	f.suggestCalls++
	if f.suggestErr != nil {
		return nil, f.suggestErr
	}
	return f.suggestions, nil
}

func (f *fakeCatalog) Owned(_ context.Context, userID, slug string) ([]store.CatalogOwnership, error) {
	f.ownedCalls = append(f.ownedCalls, userID+"|"+slug)
	return f.owned, nil
}

func strPtr(s string) *string { return &s }

func falconEntry() store.CatalogEntry {
	return store.CatalogEntry{
		ID: "11111111-1111-4111-8111-111111111111", CategorySlug: "lego-sets", CategoryName: "Lego Sets",
		Name: "Millennium Falcon", Identifier: strPtr("75192"),
		Attributes: json.RawMessage(`{"set_number":"75192"}`), Source: "seed", Status: "approved",
	}
}

func legoCollections() *fakeCollections {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "lego-sets", CategoryName: "Lego Sets", Name: "Mine",
	}
	colls.items["col-2"] = store.Collection{
		ID: "col-2", UserID: "someone-else", CategorySlug: "lego-sets", CategoryName: "Lego Sets", Name: "Theirs",
	}
	return colls
}

// ---- public browse ----------------------------------------------------------

func TestListCatalogRequiresCategory(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{})
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries", nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListCatalogUnknownCategory404(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{})
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries?category_slug=unknown", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListCatalogPassesPagingAndQuery(t *testing.T) {
	cat := &fakeCatalog{page: store.CatalogPage{
		Entries: []store.CatalogEntry{falconEntry()}, Total: 30, HasMore: true,
	}}
	r := newItemsRouterPublic(t, cat)
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries?category_slug=lego-sets&q=%20falcon%20&limit=10&offset=20", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var payload struct {
		Entries []store.CatalogEntry `json:"entries"`
		Total   int                  `json:"total"`
		HasMore bool                 `json:"has_more"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Len(t, payload.Entries, 1)
	require.Equal(t, 30, payload.Total)
	require.True(t, payload.HasMore)
	require.Equal(t, []store.CatalogQuery{{CategorySlug: "lego-sets", Q: "falcon", Limit: 10, Offset: 20}}, cat.listCalls)
}

func TestListCatalogDefaultsAndBounds(t *testing.T) {
	cat := &fakeCatalog{}
	r := newItemsRouterPublic(t, cat)

	rr := doJSON(t, r, http.MethodGet, "/catalog/entries?category_slug=lego-sets", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, store.CatalogQuery{CategorySlug: "lego-sets", Limit: 24}, cat.listCalls[0])

	for _, q := range []string{
		"category_slug=lego-sets&limit=0", "category_slug=lego-sets&limit=101", "category_slug=lego-sets&limit=x",
		"category_slug=lego-sets&offset=-1", "category_slug=lego-sets&offset=10001", "category_slug=Not%20A%20Slug",
	} {
		rr := doJSON(t, r, http.MethodGet, "/catalog/entries?"+q, nil)
		require.Equal(t, http.StatusBadRequest, rr.Code, q)
	}
}

func TestListCatalogEmpty(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{})
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries?category_slug=lego-sets&q=nothing", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"entries":[],"total":0,"has_more":false}`, rr.Body.String())
}

func TestListCatalogStoreError(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{err: errors.New("boom")})
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries?category_slug=lego-sets", nil)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetCatalogEntry(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{entries: []store.CatalogEntry{falconEntry()}})

	rr := doJSON(t, r, http.MethodGet, "/catalog/entries/11111111-1111-4111-8111-111111111111", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var got store.CatalogEntry
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "Millennium Falcon", got.Name)

	rr = doJSON(t, r, http.MethodGet, "/catalog/entries/22222222-2222-4222-8222-222222222222", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

// ---- signed-in --------------------------------------------------------------

func TestOwnedCatalogEntriesAreTheCallers(t *testing.T) {
	cat := &fakeCatalog{owned: []store.CatalogOwnership{{
		CatalogEntryID: "e1", ItemID: "i1", CollectionID: "col-1", CollectionName: "Mine",
	}}}
	r := newItemsRouter(t, legoCollections(), newFakeItems(), cat)

	rr := doJSON(t, r, http.MethodGet, "/me/catalog-ownership?category_slug=lego-sets", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t,
		`{"owned":[{"catalog_entry_id":"e1","item_id":"i1","collection_id":"col-1","collection_name":"Mine"}]}`,
		rr.Body.String())
	require.Equal(t, []string{"user-1|lego-sets"}, cat.ownedCalls)

	rr = doJSON(t, r, http.MethodGet, "/me/catalog-ownership", nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestOwnedCatalogEntriesRequiresSignIn(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{})
	rr := doJSON(t, r, http.MethodGet, "/me/catalog-ownership?category_slug=lego-sets", nil)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestCreateItemLinkedToCatalogEntry(t *testing.T) {
	items := newFakeItems()
	items.unavailableEntries = map[string]bool{"pending-entry": true}
	r := newItemsRouter(t, legoCollections(), items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", map[string]any{
		"name": "Millennium Falcon", "catalog_entry_id": "e1",
		"attributes": map[string]any{"set_number": "75192"},
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var got store.Item
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.NotNil(t, got.CatalogEntryID)
	require.Equal(t, "e1", *got.CatalogEntryID)

	rr = doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", map[string]any{
		"name": "x", "catalog_entry_id": "pending-entry",
	})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "catalog")

	rr = doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", map[string]any{
		"name": "x", "catalog_entry_id": "",
	})
	require.Equal(t, http.StatusCreated, rr.Code, "a blank id means a free-form item")
	var freeForm store.Item
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &freeForm))
	require.Nil(t, freeForm.CatalogEntryID)
}

func TestLinkAndUnlinkItem(t *testing.T) {
	items := newFakeItems()
	items.items["item-1"] = store.Item{ID: "item-1", CollectionID: "col-1", Name: "Falcon", Quantity: 1}
	items.unavailableEntries = map[string]bool{"pending-entry": true}
	r := newItemsRouter(t, legoCollections(), items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodPut, "/me/collections/col-1/items/item-1/catalog-entry",
		map[string]any{"catalog_entry_id": "e1"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var got store.Item
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "e1", *got.CatalogEntryID)

	rr = doJSON(t, r, http.MethodPut, "/me/collections/col-1/items/item-1/catalog-entry",
		map[string]any{"catalog_entry_id": "pending-entry"})
	require.Equal(t, http.StatusBadRequest, rr.Code)

	rr = doJSON(t, r, http.MethodPut, "/me/collections/col-1/items/item-1/catalog-entry",
		map[string]any{"catalog_entry_id": "  "})
	require.Equal(t, http.StatusBadRequest, rr.Code, "linking needs an entry; unlink with DELETE")

	rr = doJSON(t, r, http.MethodDelete, "/me/collections/col-1/items/item-1/catalog-entry", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var unlinked store.Item
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &unlinked))
	require.Nil(t, unlinked.CatalogEntryID)

	// The store answers ErrNotFound for other users' items (covered by the
	// store integration test); the handler maps it to 404.
	rr = doJSON(t, r, http.MethodDelete, "/me/collections/col-1/items/missing/catalog-entry", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestCatalogSuggestions(t *testing.T) {
	cat := &fakeCatalog{suggestions: []store.CatalogSuggestion{{CatalogEntry: falconEntry(), Match: store.MatchIdentifier}}}
	items := newFakeItems()
	items.items["item-1"] = store.Item{ID: "item-1", CollectionID: "col-1", Name: "Falcon", Quantity: 1}
	r := newItemsRouter(t, legoCollections(), items, cat)

	rr := doJSON(t, r, http.MethodGet, "/me/collections/col-1/items/item-1/catalog-suggestions", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var payload struct {
		Suggestions []store.CatalogSuggestion `json:"suggestions"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Len(t, payload.Suggestions, 1)
	require.Equal(t, store.MatchIdentifier, payload.Suggestions[0].Match)
	require.Equal(t, "Millennium Falcon", payload.Suggestions[0].Name)

	cat.suggestErr = store.ErrNotFound
	rr = doJSON(t, r, http.MethodGet, "/me/collections/col-2/items/item-2/catalog-suggestions", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}
