package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// fakeCollections is a lightweight in-memory stand-in for store.Collections.
// We only need to exercise handler logic: happy path, validation, 404s, 500s.
type fakeCollections struct {
	mu        sync.Mutex
	seq       int
	items     map[string]store.Collection // id -> collection
	createErr error
	listErr   error
}

func newFakeCollections() *fakeCollections {
	return &fakeCollections{items: map[string]store.Collection{}}
}

func (f *fakeCollections) Create(_ context.Context, userID, slug, name string) (store.Collection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return store.Collection{}, f.createErr
	}
	f.seq++
	id := "col-" + itoa(f.seq)
	c := store.Collection{
		ID: id, UserID: userID, CategorySlug: slug, CategoryName: slug,
		Name: name, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.items[id] = c
	return c, nil
}

func (f *fakeCollections) ListByUser(_ context.Context, userID string) ([]store.Collection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]store.Collection, 0)
	for _, c := range f.items {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCollections) GetByIDForUser(_ context.Context, id, userID string) (store.Collection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.items[id]
	if !ok || c.UserID != userID {
		return store.Collection{}, store.ErrNotFound
	}
	return c, nil
}

func (f *fakeCollections) Rename(_ context.Context, id, userID, name string) (store.Collection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.items[id]
	if !ok || c.UserID != userID {
		return store.Collection{}, store.ErrNotFound
	}
	c.Name = name
	c.UpdatedAt = time.Now()
	f.items[id] = c
	return c, nil
}

func (f *fakeCollections) Delete(_ context.Context, id, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.items[id]
	if !ok || c.UserID != userID {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

// fakeUsers implements server.UserStore for /me to return a stub user when
// the router tests care about it.
type fakeUsers struct {
	user    store.User
	errByID error
}

func (f *fakeUsers) GetByID(_ context.Context, _ string) (store.User, error) {
	if f.errByID != nil {
		return store.User{}, f.errByID
	}
	return f.user, nil
}

func itoa(i int) string {
	// avoid strconv to keep imports small; ids in tests are small positive ints
	if i == 0 {
		return "0"
	}
	out := []byte{}
	for i > 0 {
		out = append([]byte{byte('0' + i%10)}, out...)
		i /= 10
	}
	return string(out)
}

func newAuthenticatedRouter(t *testing.T, colls server.CollectionStore, userID string) http.Handler {
	t.Helper()
	return newAuthenticatedRouterWithDeps(t, server.Deps{
		Categories:  &fakeCategories{},
		Collections: colls,
		Users:       &fakeUsers{user: store.User{ID: userID, Email: "u@x", DisplayName: "U"}},
	}, userID)
}

// newAuthenticatedRouterWithDeps lets tests supply any subset of Deps and
// fills in the fiddly bits (logger, sessions, CORS, DB pinger) with sane
// defaults. Used by both collection and item handler tests.
func newAuthenticatedRouterWithDeps(t *testing.T, deps server.Deps, userID string) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessions := auth.NewManager([]byte("01234567890123456789012345678901"), false)
	if deps.Logger == nil {
		deps.Logger = logger
	}
	if deps.DBPinger == nil {
		deps.DBPinger = fakePinger{}
	}
	if deps.Sessions == nil {
		deps.Sessions = sessions
	}
	if deps.CORSOrigins == nil {
		deps.CORSOrigins = []string{"http://localhost:3000"}
	}
	return authedWrap(server.NewRouter(deps), deps.Sessions, userID)
}

// newItemsRouterPublic builds a router for unauth'd endpoints (catalog
// search) with just a fake catalog.
func newItemsRouterPublic(t *testing.T, catalog server.CatalogStore) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessions := auth.NewManager([]byte("01234567890123456789012345678901"), false)
	return server.NewRouter(server.Deps{
		Logger:      logger,
		DBPinger:    fakePinger{},
		Categories:  &fakeCategories{},
		Collections: newFakeCollections(),
		Items:       newFakeItems(),
		Catalog:     catalog,
		Users:       &fakeUsers{},
		Sessions:    sessions,
		CORSOrigins: []string{"http://localhost:3000"},
	})
}

// authedWrap wraps the router in a pre-middleware that writes a valid
// session cookie onto every incoming request. Avoids exercising the cookie
// layer inside each test — we already cover that in session_test.go.
func authedWrap(h http.Handler, m *auth.Manager, userID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr := httptest.NewRecorder()
		require := func() { _ = m.Write(rr, auth.Session{UserID: userID}) }
		require()
		for _, c := range rr.Result().Cookies() {
			r.AddCookie(c)
		}
		h.ServeHTTP(w, r)
	})
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		buf = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestCollectionsRequireAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessions := auth.NewManager([]byte("01234567890123456789012345678901"), false)
	r := server.NewRouter(server.Deps{
		Logger:      logger,
		DBPinger:    fakePinger{},
		Categories:  &fakeCategories{},
		Collections: newFakeCollections(),
		Users:       &fakeUsers{},
		Sessions:    sessions,
		CORSOrigins: []string{"http://localhost:3000"},
	})

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/me/collections"},
		{http.MethodPost, "/me/collections"},
		{http.MethodGet, "/me/collections/col-1"},
		{http.MethodPatch, "/me/collections/col-1"},
		{http.MethodDelete, "/me/collections/col-1"},
	}
	for _, tc := range paths {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			require.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestCreateMyCollectionHappyPath(t *testing.T) {
	colls := newFakeCollections()
	r := newAuthenticatedRouter(t, colls, "user-1")

	rr := doJSON(t, r, http.MethodPost, "/me/collections",
		map[string]string{"category_slug": "lego-sets", "name": "  My Lego  "})

	require.Equal(t, http.StatusCreated, rr.Code)
	var got store.Collection
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "My Lego", got.Name, "server should trim whitespace")
	require.Equal(t, "lego-sets", got.CategorySlug)
	// UserID is not serialized (json:"-"); ownership is verified by list/cross-user tests.
}

func TestCreateValidationErrors(t *testing.T) {
	colls := newFakeCollections()
	r := newAuthenticatedRouter(t, colls, "user-1")

	cases := []struct {
		name string
		body any
		want int
	}{
		{"missing name", map[string]string{"category_slug": "lego-sets", "name": ""}, http.StatusBadRequest},
		{"name too long", map[string]string{"category_slug": "lego-sets", "name": strings.Repeat("x", 101)}, http.StatusBadRequest},
		{"missing category_slug", map[string]string{"name": "x"}, http.StatusBadRequest},
		{"extra field rejected", map[string]any{"category_slug": "lego-sets", "name": "x", "sneaky": true}, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := doJSON(t, r, http.MethodPost, "/me/collections", tc.body)
			require.Equal(t, tc.want, rr.Code)
		})
	}
}

func TestCreateUnknownCategoryReturns400(t *testing.T) {
	colls := newFakeCollections()
	colls.createErr = store.ErrCategoryNotFound
	r := newAuthenticatedRouter(t, colls, "user-1")

	rr := doJSON(t, r, http.MethodPost, "/me/collections",
		map[string]string{"category_slug": "nope", "name": "x"})
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListReturnsOnlyMyCollections(t *testing.T) {
	colls := newFakeCollections()
	// seed directly so we can hand-set owners
	colls.items["col-a"] = store.Collection{ID: "col-a", UserID: "user-1", Name: "mine1"}
	colls.items["col-b"] = store.Collection{ID: "col-b", UserID: "user-1", Name: "mine2"}
	colls.items["col-c"] = store.Collection{ID: "col-c", UserID: "someone-else", Name: "not mine"}

	r := newAuthenticatedRouter(t, colls, "user-1")

	rr := doJSON(t, r, http.MethodGet, "/me/collections", nil)
	require.Equal(t, http.StatusOK, rr.Code)

	var payload struct {
		Collections []store.Collection `json:"collections"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Len(t, payload.Collections, 2)
	names := []string{payload.Collections[0].Name, payload.Collections[1].Name}
	require.Contains(t, names, "mine1")
	require.Contains(t, names, "mine2")
	require.NotContains(t, names, "not mine", "must not leak other users' collections")
}

func TestGetCrossUserReturns404(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{ID: "col-1", UserID: "other", Name: "theirs"}

	r := newAuthenticatedRouter(t, colls, "user-1")
	rr := doJSON(t, r, http.MethodGet, "/me/collections/col-1", nil)
	require.Equal(t, http.StatusNotFound, rr.Code, "cross-user get must hide existence")
}

func TestRenameAndDeleteHappyAndNotFound(t *testing.T) {
	colls := newFakeCollections()
	colls.items["col-1"] = store.Collection{ID: "col-1", UserID: "user-1", Name: "old"}
	colls.items["col-2"] = store.Collection{ID: "col-2", UserID: "someone-else", Name: "theirs"}

	r := newAuthenticatedRouter(t, colls, "user-1")

	t.Run("rename updates mine", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodPatch, "/me/collections/col-1", map[string]string{"name": "new"})
		require.Equal(t, http.StatusOK, rr.Code)
		var got store.Collection
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
		require.Equal(t, "new", got.Name)
	})

	t.Run("rename someone else's is 404", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodPatch, "/me/collections/col-2", map[string]string{"name": "hijacked"})
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("delete mine returns 204", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodDelete, "/me/collections/col-1", nil)
		require.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("delete someone else's is 404", func(t *testing.T) {
		rr := doJSON(t, r, http.MethodDelete, "/me/collections/col-2", nil)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestListSurfaces500OnStoreError(t *testing.T) {
	colls := newFakeCollections()
	colls.listErr = errors.New("db down")
	r := newAuthenticatedRouter(t, colls, "user-1")
	rr := doJSON(t, r, http.MethodGet, "/me/collections", nil)
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}
