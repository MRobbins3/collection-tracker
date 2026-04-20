package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// ---- fakes ---------------------------------------------------------------

type fakeItems struct {
	mu    sync.Mutex
	seq   int
	items map[string]store.Item // id -> item
}

func newFakeItems() *fakeItems { return &fakeItems{items: map[string]store.Item{}} }

// Ownership is validated by the fakeCollections; items here trust that the
// handler already confirmed the collection belongs to the user.
func (f *fakeItems) Create(_ context.Context, collectionID, _ string, in store.ItemInput) (store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := "item-" + itoa(f.seq)
	attrs := in.Attributes
	if len(attrs) == 0 {
		attrs = json.RawMessage("{}")
	}
	it := store.Item{
		ID: id, CollectionID: collectionID, Name: in.Name, Quantity: in.Quantity,
		Condition: in.Condition, Attributes: attrs,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.items[id] = it
	return it, nil
}

func (f *fakeItems) ListInCollectionForUser(_ context.Context, collectionID, _ string) ([]store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Item, 0)
	for _, it := range f.items {
		if it.CollectionID == collectionID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (f *fakeItems) GetInCollectionForUser(_ context.Context, id, collectionID, _ string) (store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || it.CollectionID != collectionID {
		return store.Item{}, store.ErrNotFound
	}
	return it, nil
}

func (f *fakeItems) Update(_ context.Context, id, collectionID, _ string, in store.ItemInput) (store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || it.CollectionID != collectionID {
		return store.Item{}, store.ErrNotFound
	}
	it.Name, it.Quantity, it.Condition, it.UpdatedAt = in.Name, in.Quantity, in.Condition, time.Now()
	if len(in.Attributes) > 0 {
		it.Attributes = in.Attributes
	}
	f.items[id] = it
	return it, nil
}

func (f *fakeItems) Delete(_ context.Context, id, collectionID, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || it.CollectionID != collectionID {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

type fakeCatalog struct {
	entries []store.CatalogEntry
	err     error
}

func (f *fakeCatalog) Search(_ context.Context, _ string, _ string) ([]store.CatalogEntry, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entries, nil
}

// Extend fakeCategories from router_test.go with GetBySlug that returns the
// Lego schema so attribute validation has something real to check against.
type fakeCategoriesWithSchema struct {
	fakeCategories
	schema json.RawMessage
}

func (f *fakeCategoriesWithSchema) GetBySlug(_ context.Context, slug string) (store.Category, error) {
	if slug == "lego-sets" {
		return store.Category{ID: "cat-1", Slug: "lego-sets", Name: "Lego Sets", AttributeSchema: f.schema}, nil
	}
	return store.Category{}, store.ErrNotFound
}

// ---- helpers --------------------------------------------------------------

func newItemsRouter(t *testing.T, colls server.CollectionStore, items server.ItemStore, catalog server.CatalogStore) http.Handler {
	t.Helper()
	legoSchema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"set_number": {"type": "string"},
			"piece_count": {"type": "integer", "minimum": 0}
		},
		"additionalProperties": false
	}`)
	cats := &fakeCategoriesWithSchema{schema: legoSchema}
	return newAuthenticatedRouterWithDeps(t, server.Deps{
		Categories:  cats,
		Collections: colls,
		Items:       items,
		Catalog:     catalog,
		Users:       &fakeUsers{user: store.User{ID: "user-1", Email: "u@x", DisplayName: "U"}},
	}, "user-1")
}

// ---- tests ----------------------------------------------------------------

func TestCreateItemHappyPath(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "lego-sets",
		CategoryName: "Lego Sets", Name: "My Lego",
	}
	items := newFakeItems()
	r := newItemsRouter(t, colls, items, &fakeCatalog{})

	body := map[string]any{
		"name":       "Millennium Falcon UCS",
		"quantity":   1,
		"condition":  "New, sealed",
		"attributes": map[string]any{"set_number": "75192", "piece_count": 7541},
	}
	rr := doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", body)
	require.Equal(t, http.StatusCreated, rr.Code, "body: %s", rr.Body.String())
}

func TestCreateItemRejectsBadAttributes(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "lego-sets",
		CategoryName: "Lego Sets", Name: "My Lego",
	}
	r := newItemsRouter(t, colls, newFakeItems(), &fakeCatalog{})

	body := map[string]any{
		"name":     "Bad Item",
		"quantity": 1,
		// piece_count must be an integer — string violates schema
		"attributes": map[string]any{"set_number": "75192", "piece_count": "many"},
	}
	rr := doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", body)
	require.Equal(t, http.StatusBadRequest, rr.Code, "body: %s", rr.Body.String())

	var payload struct {
		Error  string            `json:"error"`
		Fields map[string]string `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.NotEmpty(t, payload.Fields, "field-level errors expected: %s", rr.Body.String())
}

func TestCreateItemInUnownedCollection404(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "someone-else", CategorySlug: "lego-sets",
		CategoryName: "Lego Sets", Name: "theirs",
	}
	r := newItemsRouter(t, colls, newFakeItems(), &fakeCatalog{})

	body := map[string]any{"name": "x", "quantity": 1}
	rr := doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", body)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestListItemsHappyPath(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "lego-sets",
		CategoryName: "Lego Sets", Name: "mine",
	}
	items := newFakeItems()
	items.items["item-99"] = store.Item{ID: "item-99", CollectionID: "col-1", Name: "X", Quantity: 1}
	r := newItemsRouter(t, colls, items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodGet, "/me/collections/col-1/items", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var payload struct {
		Items []store.Item `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Len(t, payload.Items, 1)
	require.Equal(t, "X", payload.Items[0].Name)
}

func TestUpdateAndDeleteItem(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "lego-sets",
		CategoryName: "Lego Sets", Name: "mine",
	}
	items := newFakeItems()
	items.items["item-7"] = store.Item{ID: "item-7", CollectionID: "col-1", Name: "old", Quantity: 1}
	r := newItemsRouter(t, colls, items, &fakeCatalog{})

	t.Run("update name + quantity", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodPatch, "/me/collections/col-1/items/item-7",
			map[string]any{"name": "new", "quantity": 3})
		require.Equal(t, http.StatusOK, rr.Code)
		var got store.Item
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
		require.Equal(t, "new", got.Name)
		require.Equal(t, 3, got.Quantity)
	})

	t.Run("update bad attributes returns 400", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodPatch, "/me/collections/col-1/items/item-7",
			map[string]any{"name": "new", "quantity": 1, "attributes": map[string]any{"bogus_field": "x"}})
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("delete returns 204", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodDelete, "/me/collections/col-1/items/item-7", nil)
		require.Equal(t, http.StatusNoContent, rr.Code)
	})
}

func TestSearchCatalogRequiresCategory(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{})
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries", nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestSearchCatalogReturnsEmptyListWhenCatalogEmpty(t *testing.T) {
	r := newItemsRouterPublic(t, &fakeCatalog{entries: nil})
	rr := doJSON(t, r, http.MethodGet, "/catalog/entries?category_slug=lego-sets&q=millennium", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var payload struct {
		Entries []store.CatalogEntry `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Empty(t, payload.Entries)
}
