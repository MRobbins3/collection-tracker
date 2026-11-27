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
	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
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
	if err := cleanItemInput(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if errResp, ok := validateItemInput(body); !ok {
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}
	if status, errResp, ok := h.validateAttributes(r.Context(), s.UserID, coll.CategorySlug, body.Attributes); !ok {
		writeJSON(w, status, errResp)
		return
	}

	item, err := h.items.Create(r.Context(), collectionID, s.UserID, store.ItemInput{
		Name:           body.Name,
		Quantity:       body.Quantity,
		Condition:      body.Condition,
		Attributes:     body.Attributes,
		CatalogEntryID: trimmedOrNil(body.CatalogEntryID),
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
	if err := cleanItemInput(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if errResp, ok := validateItemInput(body); !ok {
		writeJSON(w, http.StatusBadRequest, errResp)
		return
	}
	if status, errResp, ok := h.validateAttributes(r.Context(), s.UserID, coll.CategorySlug, body.Attributes); !ok {
		writeJSON(w, status, errResp)
		return
	}

	item, err := h.items.Update(r.Context(), itemID, collectionID, s.UserID, store.ItemInput{
		Name:       body.Name,
		Quantity:   body.Quantity,
		Condition:  body.Condition,
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

// cleanItemInput applies the write-time text rules before anything is
// validated, so a padded enum value still matches its option and the length
// checks see what will be stored: the name, condition, and string attributes
// go through textnorm.Clean (case kept). A zero quantity means 1.
func cleanItemInput(in *itemInputReq) error {
	in.Name = textnorm.Clean(in.Name)
	in.Condition = cleanedOrNil(in.Condition)
	if in.Quantity == 0 {
		in.Quantity = 1
	}
	attrs, err := store.CleanAttributes(in.Attributes)
	if err != nil {
		return errors.New("attributes must be a JSON object")
	}
	in.Attributes = attrs
	return nil
}

// cleanedOrNil is textnorm.Clean for an optional field, mapping blank to nil.
func cleanedOrNil(c *string) *string {
	if c == nil {
		return nil
	}
	if cleaned := textnorm.Clean(*c); cleaned != "" {
		return &cleaned
	}
	return nil
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

// trimmedOrNil trims an optional string, mapping blank to nil. Used
// for the item condition and optional ids (catalog entry, source item).
func trimmedOrNil(c *string) *string {
	if c == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*c)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// validateAttributes checks attrs against the schema of the category the
// viewer can see under slug. On failure it returns the status and body to
// send: 400 with per-field messages, 404 when the category isn't visible
// (never skip validation), 500 when the lookup fails.
func (h *handlers) validateAttributes(ctx context.Context, viewerID, slug string, attrs json.RawMessage) (int, map[string]any, bool) {
	cat, err := h.categories.GetBySlug(ctx, viewerID, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return http.StatusNotFound, map[string]any{"error": "category not found"}, false
		}
		h.logger.Error("validateAttributes: category lookup failed", "err", err)
		return http.StatusInternalServerError, map[string]any{"error": "internal server error"}, false
	}
	err = category.ValidateAttributes(cat.AttributeSchema, attrs)
	if err == nil || errors.Is(err, category.ErrEmptySchema) {
		return 0, nil, true
	}
	var ve *category.ValidationError
	if errors.As(err, &ve) {
		return http.StatusBadRequest, map[string]any{
			"error":  ve.Summary,
			"fields": ve.Fields,
		}, false
	}
	return http.StatusBadRequest, map[string]any{"error": "attribute validation failed"}, false
}
