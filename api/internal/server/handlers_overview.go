package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// OverviewStore is the narrow view of store.Overview the HTTP layer needs.
type OverviewStore interface {
	Categories(ctx context.Context, userID string) ([]store.OverviewCategory, error)
	Items(ctx context.Context, userID string, q store.OverviewQuery) (store.OverviewItemsPage, error)
	Pin(ctx context.Context, userID, categorySlug string) error
	Unpin(ctx context.Context, userID, categorySlug string) error
	Settings(ctx context.Context, userID string) (store.OverviewSettings, error)
	UpdateSettings(ctx context.Context, userID string, s store.OverviewSettings) (store.OverviewSettings, error)
}

const (
	overviewDefaultLimit = 50
	overviewMaxLimit     = 100
	// overviewMaxOffset bounds "Show more" paging; far beyond any personal
	// collection, and it keeps a crafted offset from making Postgres skip
	// millions of rows.
	overviewMaxOffset     = 10000
	overviewMaxCategories = 20
	categorySlugMax       = 100
)

// categorySlugPattern matches the seeded slugs: lowercase words joined by
// single hyphens.
var categorySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validCategorySlug(s string) bool {
	return len(s) <= categorySlugMax && categorySlugPattern.MatchString(s)
}

// getMyOverview serves GET /me/overview: the caller's overview categories
// (with counts and pins), their show-empty setting, and one page of their
// items across every collection.
//
// Query: sort=newest|oldest|name_asc|name_desc|category (default newest);
// category=<slug>[,<slug>…] (repeatable; none means every category);
// limit=0..100 (default 50; 0 returns categories only); offset=0..10000.
func (h *handlers) getMyOverview(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	q, limit, err := parseOverviewQuery(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	cats, err := h.overview.Categories(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "getMyOverview categories failed")
		return
	}
	settings, err := h.overview.Settings(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "getMyOverview settings failed")
		return
	}
	page := store.OverviewItemsPage{Items: []store.OverviewItem{}}
	if limit > 0 {
		page, err = h.overview.Items(r.Context(), s.UserID, q)
		if err != nil {
			h.serverError(w, r, err, "getMyOverview items failed")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"categories":            cats,
		"show_empty_categories": settings.ShowEmptyCategories,
		"items":                 page.Items,
		"total":                 page.Total,
		"has_more":              page.HasMore,
	})
}

func parseOverviewQuery(v url.Values) (store.OverviewQuery, int, error) {
	q := store.OverviewQuery{Sort: store.SortNewest, Limit: overviewDefaultLimit}

	if raw := v.Get("sort"); raw != "" {
		q.Sort = store.OverviewSort(raw)
		if !store.ValidOverviewSort(q.Sort) {
			return q, 0, errors.New("sort must be one of newest, oldest, name_asc, name_desc, category")
		}
	}

	seen := map[string]bool{}
	for _, param := range v["category"] {
		for _, slug := range strings.Split(param, ",") {
			slug = strings.TrimSpace(slug)
			if slug == "" || seen[slug] {
				continue
			}
			if !validCategorySlug(slug) {
				return q, 0, errors.New("category must be a list of category slugs")
			}
			seen[slug] = true
			q.CategorySlugs = append(q.CategorySlugs, slug)
		}
	}
	if len(q.CategorySlugs) > overviewMaxCategories {
		return q, 0, errors.New("filter by at most 20 categories")
	}

	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > overviewMaxLimit {
			return q, 0, errors.New("limit must be between 0 and 100")
		}
		q.Limit = n
	}
	if raw := v.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > overviewMaxOffset {
			return q, 0, errors.New("offset must be between 0 and 10000")
		}
		q.Offset = n
	}
	return q, q.Limit, nil
}

type pinCategoryReq struct {
	CategorySlug string `json:"category_slug"`
}

// pinMyCategory serves POST /me/pinned-categories: add a category to the
// caller's overview. Idempotent; 204 on success.
func (h *handlers) pinMyCategory(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body pinCategoryReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	slug := strings.TrimSpace(body.CategorySlug)
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_slug is required"})
		return
	}
	if !validCategorySlug(slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown category"})
		return
	}
	if err := h.overview.Pin(r.Context(), s.UserID, slug); err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown category"})
			return
		}
		h.serverError(w, r, err, "pinMyCategory failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// unpinMyCategory serves DELETE /me/pinned-categories/{slug}. Idempotent:
// 204 whether or not it was pinned.
func (h *handlers) unpinMyCategory(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	slug := chi.URLParam(r, "slug")
	if validCategorySlug(slug) {
		if err := h.overview.Unpin(r.Context(), s.UserID, slug); err != nil {
			h.serverError(w, r, err, "unpinMyCategory failed")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

type overviewSettingsReq struct {
	ShowEmptyCategories *bool `json:"show_empty_categories"`
}

// patchMyOverviewSettings serves PATCH /me/overview-settings.
func (h *handlers) patchMyOverviewSettings(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body overviewSettingsReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.ShowEmptyCategories == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "show_empty_categories is required"})
		return
	}
	out, err := h.overview.UpdateSettings(r.Context(), s.UserID, store.OverviewSettings{
		ShowEmptyCategories: *body.ShowEmptyCategories,
	})
	if err != nil {
		h.serverError(w, r, err, "patchMyOverviewSettings failed")
		return
	}
	writeJSON(w, http.StatusOK, out)
}
