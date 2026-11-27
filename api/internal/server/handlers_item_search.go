package server

import (
	"net/http"
	"unicode/utf8"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

const (
	// itemSearchQueryMax matches the item name limit: a longer query can't
	// match any name.
	itemSearchQueryMax = itemNameMax
	// itemSearchLimit caps one response. No pagination yet: a search that
	// returns more than this many of your own items needs a narrower query,
	// and the UI says so when has_more is true.
	itemSearchLimit = 50
)

// searchMyItems serves GET /me/items?q=... — search across the caller's own
// items. A blank query is a normal state (the search box was cleared), so it
// returns 200 with no results rather than an error.
func (h *handlers) searchMyItems(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	// Trim and collapse whitespace, so "  star   wars " searches for "star wars".
	q := textnorm.Clean(r.URL.Query().Get("q"))
	if utf8.RuneCountInString(q) > itemSearchQueryMax {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "search must be 200 characters or fewer"})
		return
	}
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"items": []store.ItemSearchResult{}, "has_more": false})
		return
	}
	results, more, err := h.items.SearchForUser(r.Context(), s.UserID, q, itemSearchLimit)
	if err != nil {
		h.serverError(w, r, err, "searchMyItems failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": results, "has_more": more})
}
