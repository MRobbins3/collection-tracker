package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/server"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// ---- fakes ---------------------------------------------------------------

type fakeItems struct {
	mu    sync.Mutex
	seq   int
	items map[string]store.Item // id -> item

	// SearchForUser stubbing (see handlers_item_search_test.go).
	searchResults []store.ItemSearchResult
	searchMore    bool
	searchErr     error
	searchCalls   []searchCall

	// Catalog entry ids Create/SetCatalogEntry treat as unlinkable.
	unavailableEntries map[string]bool
}

func newFakeItems() *fakeItems { return &fakeItems{items: map[string]store.Item{}} }

// Ownership is validated by the fakeCollections; items here trust that the
// handler already confirmed the collection belongs to the user.
func (f *fakeItems) Create(_ context.Context, collectionID, _ string, in store.ItemInput) (store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if in.CatalogEntryID != nil && f.unavailableEntries[*in.CatalogEntryID] {
		return store.Item{}, store.ErrCatalogEntryUnavailable
	}
	f.seq++
	id := "item-" + itoa(f.seq)
	attrs := in.Attributes
	if len(attrs) == 0 {
		attrs = json.RawMessage("{}")
	}
	it := store.Item{
		ID: id, CollectionID: collectionID, Name: in.Name, Quantity: in.Quantity,
		Condition: in.Condition, Attributes: attrs, CatalogEntryID: in.CatalogEntryID,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.items[id] = it
	return it, nil
}

func (f *fakeItems) SetCatalogEntry(_ context.Context, id, collectionID, _ string, entryID *string) (store.Item, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	it, ok := f.items[id]
	if !ok || it.CollectionID != collectionID {
		return store.Item{}, store.ErrNotFound
	}
	if entryID != nil && f.unavailableEntries[*entryID] {
		return store.Item{}, store.ErrCatalogEntryUnavailable
	}
	it.CatalogEntryID = entryID
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

// Extend fakeCategories from router_test.go with GetBySlug that returns the
// Lego schema so attribute validation has something real to check against.
type fakeCategoriesWithSchema struct {
	fakeCategories
	schema json.RawMessage
}

func (f *fakeCategoriesWithSchema) GetBySlug(_ context.Context, _, slug string) (store.Category, error) {
	if slug == "lego-sets" {
		return store.Category{ID: "cat-1", Slug: "lego-sets", Name: "Lego Sets", AttributeSchema: f.schema,
			Status: store.CategoryApproved}, nil
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
			"theme": {"type": "string", "enum": ["Star Wars", "City"]},
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

// A category the caller can't see is refused, never "accept without
// validating" (ADR 0013).
func TestItemWritesRefuseAnInvisibleCategory(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "someone-elses-abc123", Name: "Odd",
	}
	items := newFakeItems()
	r := newItemsRouter(t, colls, items, &fakeCatalog{})
	rr := doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", map[string]any{
		"name": "Anything", "attributes": map[string]any{"whatever": "goes"},
	})
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), "category not found")
}

// The handler cleans the name, condition, and string attributes before it
// validates them, so a padded enum value still matches its option and the
// store never sees stray whitespace (textnorm.Clean). Case is kept.
func TestItemWritesCleanTextBeforeValidating(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{
		ID: "col-1", UserID: "user-1", CategorySlug: "lego-sets",
		CategoryName: "Lego Sets", Name: "mine",
	}
	items := newFakeItems()
	r := newItemsRouter(t, colls, items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", map[string]any{
		"name":       "  Millennium \t Falcon   UCS ",
		"condition":  " New,   sealed ",
		"attributes": map[string]any{"set_number": " 75192  ", "theme": "  Star   Wars ", "piece_count": 7541},
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var got store.Item
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "Millennium Falcon UCS", got.Name)
	require.Equal(t, "New, sealed", *got.Condition)
	require.JSONEq(t, `{"set_number":"75192","theme":"Star Wars","piece_count":7541}`, string(got.Attributes))

	rr = doJSON(t, r, http.MethodPatch, "/me/collections/col-1/items/"+got.ID, map[string]any{
		"name":       " lego   city  ",
		"attributes": map[string]any{"theme": " City "},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "lego city", got.Name, "Title Case is only a UI suggestion")
	require.JSONEq(t, `{"theme":"City"}`, string(got.Attributes))

	rr = doJSON(t, r, http.MethodPost, "/me/collections/col-1/items", map[string]any{"name": " \u00a0\t "})
	require.Equal(t, http.StatusBadRequest, rr.Code, "a name of only whitespace is blank")
}
