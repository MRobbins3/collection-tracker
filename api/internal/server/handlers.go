package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func (h *handlers) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) readyz(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		h.logger.Warn("readyz db ping failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handlers) listCategories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	var (
		cats []store.Category
		err  error
	)
	if q != "" {
		cats, err = h.categories.Search(r.Context(), q)
	} else {
		cats, err = h.categories.List(r.Context())
	}
	if err != nil {
		h.serverError(w, r, err, "listCategories failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}

func (h *handlers) getCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	cat, err := h.categories.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}
		h.serverError(w, r, err, "getCategory failed")
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

func (h *handlers) serverError(w http.ResponseWriter, _ *http.Request, err error, msg string) {
	h.logger.Error(msg, "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
