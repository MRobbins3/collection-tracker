package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/privacy"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// CompareStore matches two people's collections in a category.
// store.Compare implements it.
type CompareStore interface {
	Categories(ctx context.Context, viewerID, otherID string) ([]store.CompareCategory, error)
	Compare(ctx context.Context, q store.CompareQuery) (store.CompareResult, error)
}

const (
	compareDefaultLimit = 50
	compareMaxLimit     = 100
	// compareMaxOffset bounds paging, as for the catalog and overview.
	compareMaxOffset = 10000
	msgCompareSelf   = "You can’t compare your collection with yourself."
	// codeCategoryNotFound tells the web an unknown category apart from a
	// hidden profile (both 404).
	codeCategoryNotFound = "category_not_found"
)

// compareWithUser is GET /users/{handle}/compare: how the signed-in
// viewer's items in one category line up with the other person's, in four
// buckets (both, mine_only, theirs_only, neither). Query: category (slug;
// omitted = the category picker only), bucket (default both),
// limit=1..100 (default 50), offset=0..10000. categories (the picker) is
// filled on the first page only (offset 0), and null after.
//
// It reads the other person's collections, so the privacy rule applies
// exactly as on their profile: hidden is 404 (ADR 0009).
func (h *handlers) compareWithUser(w http.ResponseWriter, r *http.Request) {
	v, err := h.resolveView(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		h.serverError(w, r, err, "compareWithUser failed")
		return
	}
	if !v.found || !v.seesCollections {
		writeProfileNotFound(w)
		return
	}
	if v.rel == privacy.Self {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgCompareSelf})
		return
	}
	q, err := parseCompareQuery(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s, _ := auth.FromContext(r.Context())
	q.ViewerID = s.UserID
	q.OtherID = v.owner.UserID

	// The picker only matters on a first page; "Show more" skips it.
	var cats []store.CompareCategory
	if q.Offset == 0 {
		cats, err = h.compare.Categories(r.Context(), q.ViewerID, q.OtherID)
		if err != nil {
			h.serverError(w, r, err, "compareWithUser: categories failed")
			return
		}
	}
	out := map[string]any{
		"other":      toPublic(v.owner),
		"categories": cats,
		"category":   nil,
		"counts":     nil,
		"bucket":     q.Bucket,
		"items":      []store.CompareRow{},
		"has_more":   false,
	}
	if q.CategorySlug == "" {
		writeJSON(w, http.StatusOK, out)
		return
	}

	res, err := h.compare.Compare(r.Context(), q)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found", "code": codeCategoryNotFound})
		case errors.Is(err, store.ErrCompareSelf):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgCompareSelf})
		default:
			h.serverError(w, r, err, "compareWithUser: compare failed")
		}
		return
	}
	out["category"] = res.Category
	out["counts"] = res.Counts
	out["items"] = res.Items
	out["has_more"] = res.HasMore
	writeJSON(w, http.StatusOK, out)
}

func parseCompareQuery(v url.Values) (store.CompareQuery, error) {
	q := store.CompareQuery{
		CategorySlug: v.Get("category"),
		Bucket:       store.BucketBoth,
		Limit:        compareDefaultLimit,
	}
	if raw := v.Get("bucket"); raw != "" {
		b, ok := store.ParseCompareBucket(raw)
		if !ok {
			return q, errors.New("bucket must be both, mine_only, theirs_only, or neither")
		}
		q.Bucket = b
	}
	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > compareMaxLimit {
			return q, errors.New("limit must be between 1 and 100")
		}
		q.Limit = n
	}
	if raw := v.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > compareMaxOffset {
			return q, errors.New("offset must be between 0 and 10000")
		}
		q.Offset = n
	}
	return q, nil
}
