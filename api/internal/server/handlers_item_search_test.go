package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/server"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

type searchCall struct {
	userID string
	q      string
	limit  int
}

func (f *fakeItems) SearchForUser(_ context.Context, userID, q string, limit int) ([]store.ItemSearchResult, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searchCalls = append(f.searchCalls, searchCall{userID: userID, q: q, limit: limit})
	if f.searchErr != nil {
		return nil, false, f.searchErr
	}
	return f.searchResults, f.searchMore, nil
}

type searchPayload struct {
	Items   []store.ItemSearchResult `json:"items"`
	HasMore bool                     `json:"has_more"`
}

func decodeSearch(t *testing.T, rr *httptest.ResponseRecorder) searchPayload {
	t.Helper()
	var p searchPayload
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &p))
	return p
}

func TestSearchMyItemsRequiresAuth(t *testing.T) {
	r := server.NewRouter(server.Deps{
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		DBPinger:    fakePinger{},
		Categories:  &fakeCategories{},
		Collections: newFakeCollections(),
		Items:       newFakeItems(),
		Users:       &fakeUsers{},
		Sessions:    auth.NewManager([]byte("01234567890123456789012345678901"), false),
		CORSOrigins: []string{"http://localhost:3000"},
	})
	rr := doJSON(t, r, http.MethodGet, "/me/items?q=falcon", nil)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestSearchMyItemsHappyPath(t *testing.T) {
	items := newFakeItems()
	items.searchResults = []store.ItemSearchResult{{
		ID: "item-1", Name: "Millennium Falcon", Quantity: 1,
		CollectionID: "col-1", CollectionName: "Lego Shelf",
		CategorySlug: "lego-sets", CategoryName: "Lego Sets",
	}}
	items.searchMore = true
	r := newItemsRouter(t, newFakeCollections(), items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodGet, "/me/items?q=%20%20millennium%20%20%20falcon%20", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	p := decodeSearch(t, rr)
	require.Len(t, p.Items, 1)
	require.Equal(t, "Lego Shelf", p.Items[0].CollectionName)
	require.True(t, p.HasMore)

	require.Len(t, items.searchCalls, 1)
	call := items.searchCalls[0]
	require.Equal(t, "user-1", call.userID, "searches only the caller's items")
	require.Equal(t, "millennium falcon", call.q, "query is trimmed with inner whitespace collapsed")
	require.Equal(t, 50, call.limit)
}

func TestSearchMyItemsBlankQueryReturnsEmptyWithoutQuerying(t *testing.T) {
	items := newFakeItems()
	r := newItemsRouter(t, newFakeCollections(), items, &fakeCatalog{})

	for _, path := range []string{"/me/items", "/me/items?q=", "/me/items?q=%20%09%20"} {
		t.Run(path, func(t *testing.T) {
			rr := doJSON(t, r, http.MethodGet, path, nil)
			require.Equal(t, http.StatusOK, rr.Code)
			p := decodeSearch(t, rr)
			require.NotNil(t, p.Items, "items is [] not null")
			require.Empty(t, p.Items)
			require.False(t, p.HasMore)
		})
	}
	require.Empty(t, items.searchCalls)
}

func TestSearchMyItemsRejectsOverlongQuery(t *testing.T) {
	items := newFakeItems()
	r := newItemsRouter(t, newFakeCollections(), items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodGet, "/me/items?q="+strings.Repeat("a", 201), nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Empty(t, items.searchCalls)

	rr = doJSON(t, r, http.MethodGet, "/me/items?q="+strings.Repeat("é", 200), nil)
	require.Equal(t, http.StatusOK, rr.Code, "the limit counts characters, not bytes")
}

func TestSearchMyItemsSurfaces500OnStoreError(t *testing.T) {
	items := newFakeItems()
	items.searchErr = errors.New("db down")
	r := newItemsRouter(t, newFakeCollections(), items, &fakeCatalog{})

	rr := doJSON(t, r, http.MethodGet, "/me/items?q=falcon", nil)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.NotContains(t, rr.Body.String(), "db down")
}
