package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// ItemStore is the narrow view the HTTP layer needs over store.Items.
type ItemStore interface {
	Create(ctx context.Context, collectionID, userID string, in store.ItemInput) (store.Item, error)
	ListInCollectionForUser(ctx context.Context, collectionID, userID string) ([]store.Item, error)
	GetInCollectionForUser(ctx context.Context, id, collectionID, userID string) (store.Item, error)
	Update(ctx context.Context, id, collectionID, userID string, in store.ItemInput) (store.Item, error)
	Delete(ctx context.Context, id, collectionID, userID string) error
	SetCatalogEntry(ctx context.Context, id, collectionID, userID string, entryID *string) (store.Item, error)
	SearchForUser(ctx context.Context, userID, q string, limit int) ([]store.ItemSearchResult, bool, error)
}

type itemInputReq struct {
	Name       string          `json:"name"`
	Quantity   int             `json:"quantity"`
	Condition  *string         `json:"condition,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	// CatalogEntryID links a new item to a catalog entry (create only; see
	// store.ItemInput). Blank means free-form.
	CatalogEntryID *string `json:"catalog_entry_id,omitempty"`
}

const (
	itemNameMin     = 1
	itemNameMax     = 200
	itemQuantityMax = 1_000_000
)

func (h *handlers) listItems(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	collectionID := chi.URLParam(r, "id")

	items, err := h.items.ListInCollectionForUser(r.Context(), collectionID, s.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "listItems failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handlers) createItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	collectionID := chi.URLParam(r, "id")

	coll, err := h.collections.GetByIDForUser(r.Context(), collectionID, s.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "createItem: collection lookup")
		return
	}

	var body itemInputReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Quantity == 0 {
		body.Quantity = 1
	}

	if errResp, ok := validateItemInput(body); !ok {
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}
	if errResp, ok := validateAttributesAgainstCategory(h.categories, r.Context(), coll.CategorySlug, body.Attributes); !ok {
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}

	item, err := h.items.Create(r.Context(), collectionID, s.UserID, store.ItemInput{
		Name:           body.Name,
		Quantity:       body.Quantity,
		Condition:      normalizeCondition(body.Condition),
		Attributes:     body.Attributes,
		CatalogEntryID: normalizeCondition(body.CatalogEntryID),
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		if errors.Is(err, store.ErrCatalogEntryUnavailable) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": catalogEntryUnavailableMsg})
			return
		}
		h.serverError(w, r, err, "createItem failed")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *handlers) updateItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	collectionID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemID")

	coll, err := h.collections.GetByIDForUser(r.Context(), collectionID, s.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "updateItem: collection lookup")
		return
	}

	var body itemInputReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Quantity == 0 {
		body.Quantity = 1
	}
	if errResp, ok := validateItemInput(body); !ok {
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}
	if errResp, ok := validateAttributesAgainstCategory(h.categories, r.Context(), coll.CategorySlug, body.Attributes); !ok {
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}

	item, err := h.items.Update(r.Context(), itemID, collectionID, s.UserID, store.ItemInput{
		Name:       body.Name,
		Quantity:   body.Quantity,
		Condition:  normalizeCondition(body.Condition),
		Attributes: body.Attributes,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		h.serverError(w, r, err, "updateItem failed")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *handlers) deleteItem(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	collectionID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemID")

	if err := h.items.Delete(r.Context(), itemID, collectionID, s.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		h.serverError(w, r, err, "deleteItem failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateItemInput(in itemInputReq) (map[string]string, bool) {
	if n := len(in.Name); n < itemNameMin || n > itemNameMax {
		return map[string]string{"error": "name must be 1–200 characters"}, false
	}
	if in.Quantity < 0 || in.Quantity > itemQuantityMax {
		return map[string]string{"error": "quantity out of range"}, false
	}
	return nil, true
}

// normalizeCondition trims an optional string, mapping blank to nil. Used
// for the item condition and the optional catalog entry id.
func normalizeCondition(c *string) *string {
	if c == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*c)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validateAttributesAgainstCategory(cats CategoryStore, ctx context.Context, slug string, attrs json.RawMessage) (map[string]any, bool) {
	cat, err := cats.GetBySlug(ctx, slug)
	if err != nil {
		// If the category lookup fails the collection-level lookup above
		// would have failed too; treat as "no validation possible, accept."
		return nil, true
	}
	err = category.ValidateAttributes(cat.AttributeSchema, attrs)
	if err == nil || errors.Is(err, category.ErrEmptySchema) {
		return nil, true
	}
	var ve *category.ValidationError
	if errors.As(err, &ve) {
		return map[string]any{
			"error":  ve.Summary,
			"fields": ve.Fields,
		}, false
	}
	return map[string]any{"error": "attribute validation failed"}, false
}
