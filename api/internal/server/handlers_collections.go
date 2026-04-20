package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// CollectionStore is the narrow view the HTTP layer needs over store.Collections.
type CollectionStore interface {
	Create(ctx context.Context, userID, categorySlug, name string) (store.Collection, error)
	ListByUser(ctx context.Context, userID string) ([]store.Collection, error)
	GetByIDForUser(ctx context.Context, id, userID string) (store.Collection, error)
	Rename(ctx context.Context, id, userID, newName string) (store.Collection, error)
	Delete(ctx context.Context, id, userID string) error
}

type createCollectionReq struct {
	CategorySlug string `json:"category_slug"`
	Name         string `json:"name"`
}

type renameCollectionReq struct {
	Name string `json:"name"`
}

const (
	collectionNameMin = 1
	collectionNameMax = 100
)

func (h *handlers) listMyCollections(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	cs, err := h.collections.ListByUser(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "listMyCollections failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"collections": cs})
}

func (h *handlers) createMyCollection(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())

	var body createCollectionReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	body.CategorySlug = strings.TrimSpace(body.CategorySlug)

	if err := validateCollectionName(body.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.CategorySlug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_slug is required"})
		return
	}

	coll, err := h.collections.Create(r.Context(), s.UserID, body.CategorySlug, body.Name)
	if err != nil {
		if errors.Is(err, store.ErrCategoryNotFound) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown category"})
			return
		}
		h.serverError(w, r, err, "createMyCollection failed")
		return
	}
	writeJSON(w, http.StatusCreated, coll)
}

func (h *handlers) getMyCollection(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	coll, err := h.collections.GetByIDForUser(r.Context(), id, s.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "getMyCollection failed")
		return
	}
	writeJSON(w, http.StatusOK, coll)
}

func (h *handlers) renameMyCollection(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")

	var body renameCollectionReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if err := validateCollectionName(body.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	coll, err := h.collections.Rename(r.Context(), id, s.UserID, body.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "renameMyCollection failed")
		return
	}
	writeJSON(w, http.StatusOK, coll)
}

func (h *handlers) deleteMyCollection(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.collections.Delete(r.Context(), id, s.UserID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "deleteMyCollection failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateCollectionName(s string) error {
	if n := len(s); n < collectionNameMin || n > collectionNameMax {
		return errors.New("name must be 1–100 characters")
	}
	return nil
}

// decodeJSONBody keeps JSON parsing tight: small size cap, disallows unknown
// fields, and surfaces friendly errors.
func decodeJSONBody(body io.Reader, dst any) error {
	dec := json.NewDecoder(io.LimitReader(body, 16*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.New("invalid JSON body")
	}
	return nil
}
