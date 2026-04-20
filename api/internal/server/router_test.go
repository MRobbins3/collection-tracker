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

	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

type fakeCategories struct {
	list       []store.Category
	searchCall string
	listErr    error
	searchErr  error
	getErr     error
}

func (f *fakeCategories) List(_ context.Context) ([]store.Category, error) {
	return f.list, f.listErr
}
func (f *fakeCategories) Search(_ context.Context, q string) ([]store.Category, error) {
	f.searchCall = q
	return f.list, f.searchErr
}
func (f *fakeCategories) GetBySlug(_ context.Context, slug string) (store.Category, error) {
	if f.getErr != nil {
		return store.Category{}, f.getErr
	}
	for _, c := range f.list {
		if c.Slug == slug {
			return c, nil
		}
	}
	return store.Category{}, store.ErrNotFound
}

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(_ context.Context) error { return f.err }

func newRouterWith(cats *fakeCategories, db server.DBPinger) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return server.NewRouter(server.Deps{
		Logger:      logger,
		DBPinger:    db,
		Categories:  cats,
		CORSOrigins: []string{"http://localhost:3000"},
	})
}

func sampleCategories() []store.Category {
	desc := "Official Lego sets"
	return []store.Category{
		{ID: "11111111-1111-1111-1111-111111111111", Slug: "lego-sets", Name: "Lego Sets", Description: &desc, AttributeSchema: json.RawMessage(`{"type":"object"}`)},
		{ID: "22222222-2222-2222-2222-222222222222", Slug: "funko-pops", Name: "Funko Pops", AttributeSchema: json.RawMessage(`{"type":"object"}`)},
	}
}

func TestRouter(t *testing.T) {
	cases := []struct {
		name          string
		method        string
		path          string
		wantStatus    int
		wantBodyIncl  string
		wantContentCT string
	}{
		{"healthz GET returns ok", http.MethodGet, "/healthz", http.StatusOK, `"status":"ok"`, "application/json"},
		{"healthz POST method not allowed", http.MethodPost, "/healthz", http.StatusMethodNotAllowed, "", ""},
		{"unknown route is 404", http.MethodGet, "/does-not-exist", http.StatusNotFound, "", ""},
	}

	r := newRouterWith(&fakeCategories{list: sampleCategories()}, fakePinger{})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
			if tc.wantBodyIncl != "" {
				body, _ := io.ReadAll(rr.Body)
				if !strings.Contains(string(body), tc.wantBodyIncl) {
					t.Fatalf("body = %q, want contains %q", string(body), tc.wantBodyIncl)
				}
			}
			if tc.wantContentCT != "" {
				if ct := rr.Header().Get("Content-Type"); ct != tc.wantContentCT {
					t.Fatalf("content-type = %q, want %q", ct, tc.wantContentCT)
				}
			}
		})
	}
}

func TestRequestIDHeaderIsSet(t *testing.T) {
	r := newRouterWith(&fakeCategories{}, fakePinger{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if got := rr.Header().Get("X-Request-Id"); got == "" {
		t.Fatalf("expected X-Request-Id header to be set by middleware")
	}
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	r := newRouterWith(&fakeCategories{}, fakePinger{})

	// Preflight
	req := httptest.NewRequest(http.MethodOptions, "/categories", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("ACAO = %q, want http://localhost:3000", got)
	}

	// Actual GET
	req = httptest.NewRequest(http.MethodGet, "/categories", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("ACAO on GET = %q, want http://localhost:3000", got)
	}
}

func TestCORSRejectsUnknownOrigin(t *testing.T) {
	r := newRouterWith(&fakeCategories{}, fakePinger{})

	req := httptest.NewRequest(http.MethodOptions, "/categories", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("ACAO = %q, want empty for unknown origin", got)
	}
}

func TestReadyzReflectsDBPing(t *testing.T) {
	cases := []struct {
		name       string
		pinger     fakePinger
		wantStatus int
	}{
		{"db ok", fakePinger{}, http.StatusOK},
		{"db unavailable", fakePinger{err: errors.New("boom")}, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRouterWith(&fakeCategories{}, tc.pinger)
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
		})
	}
}

func TestListCategoriesHappy(t *testing.T) {
	cats := sampleCategories()
	r := newRouterWith(&fakeCategories{list: cats}, fakePinger{})
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var payload struct {
		Categories []store.Category `json:"categories"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v, body=%s", err, rr.Body.String())
	}
	if len(payload.Categories) != len(cats) {
		t.Fatalf("got %d categories, want %d", len(payload.Categories), len(cats))
	}
}

func TestListCategoriesWithSearchDelegates(t *testing.T) {
	fake := &fakeCategories{list: sampleCategories()[:1]}
	r := newRouterWith(fake, fakePinger{})
	req := httptest.NewRequest(http.MethodGet, "/categories?q=lego", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if fake.searchCall != "lego" {
		t.Fatalf("expected Search to be called with %q, got %q", "lego", fake.searchCall)
	}
}

func TestListCategoriesStoreError(t *testing.T) {
	fake := &fakeCategories{listErr: errors.New("db down")}
	r := newRouterWith(fake, fakePinger{})
	req := httptest.NewRequest(http.MethodGet, "/categories", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
}

func TestGetCategoryBySlug(t *testing.T) {
	cats := sampleCategories()
	r := newRouterWith(&fakeCategories{list: cats}, fakePinger{})

	t.Run("found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories/lego-sets", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var got store.Category
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Slug != "lego-sets" {
			t.Fatalf("slug = %q, want lego-sets", got.Slug)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories/mystery", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}
