package server

import (
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// User-built categories (ADR 0013). A user composes a category from fields
// in the library (drawn from approved categories), uses it privately at
// once, and can ask for it to join the public list.

const (
	categoryNameMax        = 60
	categoryDescriptionMax = 300
	// categoryBodyMax fits MaxFields fields with MaxOptions options each.
	categoryBodyMax = 128 * 1024

	msgCategoryLocked = "This category is waiting for review or already on the public list, so it can't be changed."
)

type categoryReq struct {
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	Fields      []category.Choice `json:"fields"`
	// CatalogIdentifier is the ID of the chosen field that, with the name,
	// tells catalog entries apart. Optional.
	CatalogIdentifier string `json:"catalog_identifier,omitempty"`
}

type categorySubmitReq struct {
	Note *string `json:"note,omitempty"`
}

// listCategoryFields serves GET /category-fields (public): the field
// library, de-duplicated by normalized title and type, with the categories
// that use each field and their options.
func (h *handlers) listCategoryFields(w http.ResponseWriter, r *http.Request) {
	srcs, err := h.categories.ApprovedSources(r.Context())
	if err != nil {
		h.serverError(w, r, err, "listCategoryFields failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fields": category.Library(srcs)})
}

// listMyCategories serves GET /me/categories: the categories the caller
// built, whatever their status.
func (h *handlers) listMyCategories(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	cats, err := h.categories.ListMine(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "listMyCategories failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
}

// createMyCategory serves POST /me/categories: 201 with the new private
// category; 409 with the existing one when the name matches a category the
// caller can already use.
func (h *handlers) createMyCategory(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body categoryReq
	if err := decodeJSONBodyLimit(r.Body, &body, categoryBodyMax); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	in, ok := h.buildCategoryInput(w, r, body, nil)
	if !ok {
		return
	}
	cat, err := h.categories.Create(r.Context(), s.UserID, in)
	if err != nil {
		h.writeCategoryError(w, r, err, "createMyCategory failed")
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

// updateMyCategory serves PATCH /me/categories/{slug}: replaces the name,
// description, and fields of the caller's category while it is private or
// rejected. Another user's category is 404.
func (h *handlers) updateMyCategory(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	slug := chi.URLParam(r, "slug")
	var body categoryReq
	if err := decodeJSONBodyLimit(r.Body, &body, categoryBodyMax); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	cur, err := h.categories.GetBySlug(r.Context(), s.UserID, slug)
	if err != nil || !cur.Mine {
		if err == nil || errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}
		h.serverError(w, r, err, "updateMyCategory: lookup failed")
		return
	}
	if cur.Status != store.CategoryPrivate && cur.Status != store.CategoryRejected {
		writeJSON(w, http.StatusConflict, map[string]string{"error": msgCategoryLocked})
		return
	}
	in, ok := h.buildCategoryInput(w, r, body, cur.AttributeSchema)
	if !ok {
		return
	}
	cat, err := h.categories.Update(r.Context(), s.UserID, slug, in)
	if err != nil {
		h.writeCategoryError(w, r, err, "updateMyCategory failed")
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// submitMyCategory serves POST /me/categories/{slug}/submit: ask admins to
// put the caller's category on the public list.
func (h *handlers) submitMyCategory(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body categorySubmitReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	note, msg := cleanNote(body.Note, "note", false)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	sub, err := h.moderation.SubmitCategory(r.Context(), s.UserID, chi.URLParam(r, "slug"), note)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
		case errors.Is(err, store.ErrCategoryLocked):
			writeJSON(w, http.StatusConflict, map[string]string{"error": msgCategoryLocked})
		case errors.Is(err, store.ErrSubmissionLimit):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": msgSubmissionLimit})
		default:
			h.serverError(w, r, err, "submitMyCategory failed")
		}
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

// buildCategoryInput validates the request and composes the schema from
// the field library (plus the fields already in current, when editing).
// It writes the 400 itself and reports false.
func (h *handlers) buildCategoryInput(w http.ResponseWriter, r *http.Request, body categoryReq, current []byte) (store.CategoryInput, bool) {
	name := textnorm.Clean(body.Name)
	if name == "" || utf8.RuneCountInString(name) > categoryNameMax {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Give the category a name of 1–%d characters.", categoryNameMax)})
		return store.CategoryInput{}, false
	}
	var desc *string
	if body.Description != nil {
		d := textnorm.Clean(*body.Description)
		if utf8.RuneCountInString(d) > categoryDescriptionMax {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("Keep the description to %d characters.", categoryDescriptionMax)})
			return store.CategoryInput{}, false
		}
		desc = &d
	}
	srcs, err := h.categories.ApprovedSources(r.Context())
	if err != nil {
		h.serverError(w, r, err, "category field library failed")
		return store.CategoryInput{}, false
	}
	composed, err := category.Compose(category.Library(srcs), current, body.Fields, body.CatalogIdentifier)
	if err != nil {
		var ce *category.ComposeError
		if errors.As(err, &ce) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": ce.Message})
			return store.CategoryInput{}, false
		}
		h.serverError(w, r, err, "compose category failed")
		return store.CategoryInput{}, false
	}
	return store.CategoryInput{
		Name:              name,
		Description:       desc,
		Schema:            composed.Schema,
		CatalogIdentifier: composed.CatalogIdentifier,
	}, true
}

func (h *handlers) writeCategoryError(w http.ResponseWriter, r *http.Request, err error, msg string) {
	var exists *store.CategoryExistsError
	var inUse *store.SchemaInUseError
	switch {
	case errors.As(err, &exists):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": fmt.Sprintf("You can already use a category called %q.", exists.Existing.Name),
			"existing": map[string]string{
				"slug": exists.Existing.Slug,
				"name": exists.Existing.Name,
			},
		})
	case errors.As(err, &inUse):
		writeJSON(w, http.StatusConflict, map[string]string{"error": inUse.Message})
	case errors.Is(err, store.ErrCategoryLocked):
		writeJSON(w, http.StatusConflict, map[string]string{"error": msgCategoryLocked})
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
	default:
		h.serverError(w, r, err, msg)
	}
}
