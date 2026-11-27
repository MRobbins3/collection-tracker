package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
	"github.com/MRobbins3/collection-tracker/api/internal/textnorm"
)

// CatalogStore is the narrow view of the global catalog (ADR 0004) the HTTP
// layer needs. Every read returns approved entries only.
type CatalogStore interface {
	List(ctx context.Context, q store.CatalogQuery) (store.CatalogPage, error)
	Get(ctx context.Context, id string) (store.CatalogEntry, error)
	Suggest(ctx context.Context, itemID, collectionID, userID string, limit int) ([]store.CatalogSuggestion, error)
	Owned(ctx context.Context, userID, categorySlug string) ([]store.CatalogOwnership, error)
}

const (
	catalogDefaultLimit = 24
	catalogMaxLimit     = 100
	// catalogMaxOffset bounds "Show more" paging so a crafted offset can't
	// make Postgres skip millions of rows.
	catalogMaxOffset   = 10000
	catalogQueryMax    = 200
	catalogSuggestions = 5

	catalogEntryUnavailableMsg = "That catalog entry isn't available for this collection."
)

// listCatalog serves GET /catalog/entries (public): one page of approved
// entries in a category.
//
// Query: category_slug (required); q (matches name or identifier);
// limit=1..100 (default 24); offset=0..10000.
func (h *handlers) listCatalog(w http.ResponseWriter, r *http.Request) {
	q, err := parseCatalogQuery(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	q.ViewerID = sessionUserID(r.Context())
	page, err := h.catalog.List(r.Context(), q)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}
		h.serverError(w, r, err, "listCatalog failed")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseCatalogQuery(v url.Values) (store.CatalogQuery, error) {
	q := store.CatalogQuery{
		CategorySlug: strings.TrimSpace(v.Get("category_slug")),
		Q:            textnorm.Clean(v.Get("q")),
		Limit:        catalogDefaultLimit,
	}
	if q.CategorySlug == "" {
		return q, errors.New("category_slug is required")
	}
	if !validCategorySlug(q.CategorySlug) {
		return q, errors.New("category_slug must be a category slug")
	}
	if len(q.Q) > catalogQueryMax {
		return q, errors.New("q must be at most 200 characters")
	}
	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > catalogMaxLimit {
			return q, errors.New("limit must be between 1 and 100")
		}
		q.Limit = n
	}
	if raw := v.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > catalogMaxOffset {
			return q, errors.New("offset must be between 0 and 10000")
		}
		q.Offset = n
	}
	return q, nil
}

// getCatalogEntry serves GET /catalog/entries/{entryID} (public). Entries
// that aren't approved are 404, like unknown ones.
func (h *handlers) getCatalogEntry(w http.ResponseWriter, r *http.Request) {
	e, err := h.catalog.Get(r.Context(), chi.URLParam(r, "entryID"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "catalog entry not found"})
			return
		}
		h.serverError(w, r, err, "getCatalogEntry failed")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// listMyCatalogOwnership serves GET /me/catalog-ownership?category_slug=:
// which catalog entries in the category the caller has linked items for,
// and where those items live. Feeds the "In your collection" marker.
func (h *handlers) listMyCatalogOwnership(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	slug := strings.TrimSpace(r.URL.Query().Get("category_slug"))
	if !validCategorySlug(slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_slug is required"})
		return
	}
	owned, err := h.catalog.Owned(r.Context(), s.UserID, slug)
	if err != nil {
		h.serverError(w, r, err, "listMyCatalogOwnership failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"owned": owned})
}

// suggestCatalogEntries serves
// GET /me/collections/{id}/items/{itemID}/catalog-suggestions: likely
// catalog matches for one of the caller's items. Suggestions only; linking
// is a separate, explicit request.
func (h *handlers) suggestCatalogEntries(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	got, err := h.catalog.Suggest(r.Context(), chi.URLParam(r, "itemID"), chi.URLParam(r, "id"), s.UserID, catalogSuggestions)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		h.serverError(w, r, err, "suggestCatalogEntries failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": got})
}

type catalogLinkReq struct {
	CatalogEntryID string `json:"catalog_entry_id"`
}

// linkItemToCatalog serves PUT /me/collections/{id}/items/{itemID}/catalog-entry.
// It only sets the link; the item's name and attributes are left as the
// owner wrote them.
func (h *handlers) linkItemToCatalog(w http.ResponseWriter, r *http.Request) {
	var body catalogLinkReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	entryID := strings.TrimSpace(body.CatalogEntryID)
	if entryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "catalog_entry_id is required"})
		return
	}
	h.setItemCatalogEntry(w, r, &entryID)
}

// unlinkItemFromCatalog serves DELETE …/catalog-entry: the item becomes
// free-form again, keeping its name and attributes.
func (h *handlers) unlinkItemFromCatalog(w http.ResponseWriter, r *http.Request) {
	h.setItemCatalogEntry(w, r, nil)
}

func (h *handlers) setItemCatalogEntry(w http.ResponseWriter, r *http.Request, entryID *string) {
	s, _ := auth.FromContext(r.Context())
	item, err := h.items.SetCatalogEntry(r.Context(), chi.URLParam(r, "itemID"), chi.URLParam(r, "id"), s.UserID, entryID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		case errors.Is(err, store.ErrCatalogEntryUnavailable):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": catalogEntryUnavailableMsg})
		default:
			h.serverError(w, r, err, "setItemCatalogEntry failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, item)
}
