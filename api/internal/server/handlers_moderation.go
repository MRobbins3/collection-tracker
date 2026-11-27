package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// ModerationStore is the narrow view of user submissions, corrections, and
// the admin queue (ADR 0012) the HTTP layer needs.
type ModerationStore interface {
	Submit(ctx context.Context, userID string, in store.SubmissionInput) (store.SubmitResult, error)
	ProposeCorrection(ctx context.Context, userID, entryID string, proposed store.EntryFields, note *string) (store.Submission, error)
	ListMine(ctx context.Context, userID string) ([]store.Submission, error)
	Queue(ctx context.Context, q store.QueueQuery) (store.QueuePage, error)
	Get(ctx context.Context, id string) (store.ModerationRequest, error)
	Approve(ctx context.Context, adminID, id string, edits *store.EntryFields) (store.ModerationRequest, error)
	Reject(ctx context.Context, adminID, id, reason string) (store.ModerationRequest, error)
	Merge(ctx context.Context, adminID, id, targetID string, reason *string) (store.ModerationRequest, error)
	SubmitCategory(ctx context.Context, userID, slug string, note *string) (store.Submission, error)
}

const (
	entryNameMax        = 200
	entryDescriptionMax = 1000
	moderationNoteMax   = 500
	queueDefaultLimit   = 25
	queueMaxLimit       = 100
	queueMaxOffset      = 10000
)

var (
	msgSubmissionLimit = fmt.Sprintf(
		"You have %d suggestions waiting for review. You can send more once some of them are reviewed.",
		store.MaxPendingPerUser)
	msgNoChanges         = "That doesn't change anything in the catalog entry."
	msgAlreadyDecided    = "Someone already decided this one. Reload to see the outcome."
	msgDuplicateEntry    = "Another catalog entry in this category already has that name and identifier."
	msgMergeTarget       = "Pick an approved catalog entry in the same category to merge into."
	msgWrongKind         = "Only new entry suggestions can be merged."
	msgEntryNotPending   = "This suggestion's catalog entry was changed outside the queue. Reload to see it."
	msgCategoryNotPublic = "Catalog suggestions open once this category is on the public list."
	msgCategoryNoEdits   = "Categories are approved as their creator built them. Reject with a reason to ask for changes."
	msgDuplicateCategory = "A category with this name is already on the public list. Reject this one as a duplicate."
)

// entryFieldsReq is the editable part of a catalog entry in a request
// body. Absent fields are nil.
type entryFieldsReq struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
}

// clean normalizes and validates the fields (attributes against the
// category's schema). It returns an error body for a 400.
func (h *handlers) cleanEntryFields(ctx context.Context, viewerID, categorySlug string, in entryFieldsReq) (store.EntryFields, int, map[string]any, bool) {
	var out store.EntryFields
	if in.Name != nil {
		n := textnorm.Clean(*in.Name)
		if n == "" || utf8.RuneCountInString(n) > entryNameMax {
			return out, http.StatusBadRequest, map[string]any{"error": "name must be 1–200 characters"}, false
		}
		out.Name = &n
	}
	if in.Description != nil {
		d := textnorm.Clean(*in.Description)
		if utf8.RuneCountInString(d) > entryDescriptionMax {
			return out, http.StatusBadRequest, map[string]any{"error": "description must be at most 1000 characters"}, false
		}
		out.Description = &d
	}
	if in.Attributes != nil && string(in.Attributes) != "null" {
		if status, errResp, ok := h.validateAttributes(ctx, viewerID, categorySlug, in.Attributes); !ok {
			return out, status, errResp, false
		}
		out.Attributes = in.Attributes
	}
	return out, 0, nil, true
}

func cleanNote(s *string, field string, required bool) (*string, string) {
	if s != nil {
		c := textnorm.Clean(*s)
		if utf8.RuneCountInString(c) > moderationNoteMax {
			return nil, field + " must be at most 500 characters"
		}
		if c != "" {
			return &c, ""
		}
	}
	if required {
		return nil, field + " is required"
	}
	return nil, ""
}

type submissionReq struct {
	CategorySlug string          `json:"category_slug"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	Attributes   json.RawMessage `json:"attributes,omitempty"`
	// ItemID is the caller's free-form item this suggestion came from; it
	// is linked to the entry once the entry is in the catalog.
	ItemID *string `json:"item_id,omitempty"`
}

// submitCatalogEntry serves POST /me/catalog-submissions: suggest a new
// catalog entry. 201 when queued for review; 200 when it matched an
// existing approved or pending entry instead (see store.SubmitOutcome).
func (h *handlers) submitCatalogEntry(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body submissionReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	slug := strings.TrimSpace(body.CategorySlug)
	if !validCategorySlug(slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category_slug is required"})
		return
	}
	cat, err := h.categories.GetBySlug(r.Context(), s.UserID, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "category not found"})
			return
		}
		h.serverError(w, r, err, "submitCatalogEntry: category lookup")
		return
	}
	// The catalog belongs to the public list: a user-built category gets
	// one once it is approved (ADR 0013).
	if cat.Status != store.CategoryApproved {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgCategoryNotPublic})
		return
	}
	// Omitted attributes are validated as {} (like item create), so a
	// category with required attributes can't be bypassed.
	attrs := body.Attributes
	if len(attrs) == 0 || string(attrs) == "null" {
		attrs = json.RawMessage(`{}`)
	}
	fields, status, errResp, ok := h.cleanEntryFields(r.Context(), s.UserID, slug, entryFieldsReq{
		Name: &body.Name, Description: body.Description, Attributes: attrs,
	})
	if !ok {
		writeJSON(w, status, errResp)
		return
	}
	res, err := h.moderation.Submit(r.Context(), s.UserID, store.SubmissionInput{
		CategorySlug: slug,
		Name:         *fields.Name,
		Description:  fields.Description,
		Attributes:   fields.Attributes,
		SourceItemID: trimmedOrNil(body.ItemID),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		case errors.Is(err, store.ErrSubmissionLimit):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": msgSubmissionLimit})
		default:
			h.serverError(w, r, err, "submitCatalogEntry failed")
		}
		return
	}
	status = http.StatusOK
	if res.Outcome == store.OutcomeSubmitted {
		status = http.StatusCreated
	}
	writeJSON(w, status, res)
}

// listMySubmissions serves GET /me/catalog-submissions: the caller's own
// suggestions and corrections with their outcomes ("notifications-lite").
func (h *handlers) listMySubmissions(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	subs, err := h.moderation.ListMine(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "listMySubmissions failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"submissions": subs})
}

type correctionReq struct {
	CatalogEntryID string `json:"catalog_entry_id"`
	entryFieldsReq
	Note *string `json:"note,omitempty"`
}

// proposeCatalogCorrection serves POST /me/catalog-corrections: propose
// changes to an approved entry. Nothing changes until an admin approves.
func (h *handlers) proposeCatalogCorrection(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body correctionReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	entryID := strings.TrimSpace(body.CatalogEntryID)
	if entryID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "catalog_entry_id is required"})
		return
	}
	if body.Name == nil && body.Description == nil && body.Attributes == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgNoChanges})
		return
	}
	note, msg := cleanNote(body.Note, "note", false)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	entry, err := h.catalog.Get(r.Context(), entryID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "catalog entry not found"})
			return
		}
		h.serverError(w, r, err, "proposeCatalogCorrection: entry lookup")
		return
	}
	fields, status, errResp, ok := h.cleanEntryFields(r.Context(), s.UserID, entry.CategorySlug, body.entryFieldsReq)
	if !ok {
		writeJSON(w, status, errResp)
		return
	}
	sub, err := h.moderation.ProposeCorrection(r.Context(), s.UserID, entryID, fields, note)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "catalog entry not found"})
		case errors.Is(err, store.ErrNoChanges):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgNoChanges})
		case errors.Is(err, store.ErrSubmissionLimit):
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": msgSubmissionLimit})
		default:
			h.serverError(w, r, err, "proposeCatalogCorrection failed")
		}
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

// ---- admin ------------------------------------------------------------------

// listModerationQueue serves GET /admin/moderation (admins): pending
// requests, oldest first.
//
// Query: kind (catalog_entry | catalog_correction); category_slug;
// limit=1..100 (default 25); offset=0..10000.
func (h *handlers) listModerationQueue(w http.ResponseWriter, r *http.Request) {
	q, err := parseQueueQuery(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	page, err := h.moderation.Queue(r.Context(), q)
	if err != nil {
		h.serverError(w, r, err, "listModerationQueue failed")
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func parseQueueQuery(v url.Values) (store.QueueQuery, error) {
	q := store.QueueQuery{Limit: queueDefaultLimit}
	switch kind := store.ModerationKind(v.Get("kind")); kind {
	case "", store.KindCatalogEntry, store.KindCatalogCorrection, store.KindCategory:
		q.Kind = kind
	default:
		return q, errors.New("kind must be catalog_entry, catalog_correction, or category")
	}
	if slug := strings.TrimSpace(v.Get("category_slug")); slug != "" {
		if !validCategorySlug(slug) {
			return q, errors.New("category_slug must be a category slug")
		}
		q.CategorySlug = slug
	}
	if raw := v.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > queueMaxLimit {
			return q, errors.New("limit must be between 1 and 100")
		}
		q.Limit = n
	}
	if raw := v.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > queueMaxOffset {
			return q, errors.New("offset must be between 0 and 10000")
		}
		q.Offset = n
	}
	return q, nil
}

// getModerationRequest serves GET /admin/moderation/{requestID}.
func (h *handlers) getModerationRequest(w http.ResponseWriter, r *http.Request) {
	req, ok := h.loadModerationRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *handlers) loadModerationRequest(w http.ResponseWriter, r *http.Request) (store.ModerationRequest, bool) {
	req, err := h.moderation.Get(r.Context(), chi.URLParam(r, "requestID"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "request not found"})
			return store.ModerationRequest{}, false
		}
		h.serverError(w, r, err, "loadModerationRequest failed")
		return store.ModerationRequest{}, false
	}
	return req, true
}

// approveModerationRequest serves POST /admin/moderation/{requestID}/approve.
// An empty body approves as submitted; any of name, description, and
// attributes approve the admin's edited version instead.
func (h *handlers) approveModerationRequest(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body entryFieldsReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	req, ok := h.loadModerationRequest(w, r)
	if !ok {
		return
	}
	var edits *store.EntryFields
	if body.Name != nil || body.Description != nil || body.Attributes != nil {
		if req.Kind == store.KindCategory {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgCategoryNoEdits})
			return
		}
		fields, status, errResp, ok := h.cleanEntryFields(r.Context(), s.UserID, req.CategorySlug, body)
		if !ok {
			writeJSON(w, status, errResp)
			return
		}
		edits = &fields
	}
	out, err := h.moderation.Approve(r.Context(), s.UserID, req.ID, edits)
	h.writeDecision(w, r, out, err)
}

type rejectReq struct {
	Reason *string `json:"reason"`
}

// rejectModerationRequest serves POST /admin/moderation/{requestID}/reject.
// The reason is required: the submitter sees it in "My submissions".
func (h *handlers) rejectModerationRequest(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body rejectReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	reason, msg := cleanNote(body.Reason, "reason", true)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	out, err := h.moderation.Reject(r.Context(), s.UserID, chi.URLParam(r, "requestID"), *reason)
	h.writeDecision(w, r, out, err)
}

type mergeReq struct {
	CatalogEntryID string  `json:"catalog_entry_id"`
	Reason         *string `json:"reason,omitempty"`
}

// mergeModerationRequest serves POST /admin/moderation/{requestID}/merge:
// a duplicate suggestion resolves to an existing approved entry.
func (h *handlers) mergeModerationRequest(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body mergeReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	target := strings.TrimSpace(body.CatalogEntryID)
	if target == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "catalog_entry_id is required"})
		return
	}
	reason, msg := cleanNote(body.Reason, "reason", false)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	out, err := h.moderation.Merge(r.Context(), s.UserID, chi.URLParam(r, "requestID"), target, reason)
	h.writeDecision(w, r, out, err)
}

func (h *handlers) writeDecision(w http.ResponseWriter, r *http.Request, out store.ModerationRequest, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "request not found"})
	case errors.Is(err, store.ErrAlreadyDecided):
		writeJSON(w, http.StatusConflict, map[string]string{"error": msgAlreadyDecided})
	case errors.Is(err, store.ErrDuplicateEntry):
		writeJSON(w, http.StatusConflict, map[string]string{"error": msgDuplicateEntry})
	case errors.Is(err, store.ErrDuplicateCategory):
		writeJSON(w, http.StatusConflict, map[string]string{"error": msgDuplicateCategory})
	case errors.Is(err, store.ErrMergeTarget):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgMergeTarget})
	case errors.Is(err, store.ErrWrongKind):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgWrongKind})
	case errors.Is(err, store.ErrEntryNotPending):
		writeJSON(w, http.StatusConflict, map[string]string{"error": msgEntryNotPending})
	default:
		h.serverError(w, r, err, "moderation decision failed")
	}
}
