package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/profile"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// RelationshipStore resolves how a viewer relates to an owner for the
// privacy rule. store.Relationships implements it.
type RelationshipStore interface {
	Between(ctx context.Context, viewerID, ownerID string) (privacy.Relationship, error)
}

// selfOrNone is the fallback when no RelationshipStore is wired (tests that
// don't care): it knows only "that's me" and "stranger".
type selfOrNone struct{}

func (selfOrNone) Between(_ context.Context, viewerID, ownerID string) (privacy.Relationship, error) {
	return privacy.RelationshipFor(viewerID, ownerID, privacy.None), nil
}

// Handle checks are rate limited per user: unique handles make the check an
// unavoidable "does @x exist?" oracle (even for Private profiles; ADR 0009),
// so it must not be usable for bulk enumeration. 20 quick checks, then one
// every 3 seconds, is plenty for someone typing.
const (
	handleCheckBurst    = 20
	handleCheckInterval = 3 * time.Second
	msgTooManyChecks    = "You’re checking handles too quickly. Wait a moment and try again."
)

const (
	msgFixFields  = "Please fix the highlighted fields."
	msgVisibility = "Choose who can see your collections."
	msgConfirm    = "Type your handle exactly to confirm."
)

// profilePatchReq is PATCH /me/profile. Every field is optional; omitted
// fields are left as they are.
type profilePatchReq struct {
	Handle               *string `json:"handle"`
	DisplayName          *string `json:"display_name"`
	Bio                  *string `json:"bio"`
	CollectionVisibility *string `json:"collection_visibility"`
	AcceptFriendRequests *bool   `json:"accept_friend_requests"`
	AcceptFollowRequests *bool   `json:"accept_follow_requests"`
}

// publicProfile is what other people may see of a profile. Email, privacy
// toggles, and internal ids are deliberately absent.
type publicProfile struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarColor string `json:"avatar_color"`
}

func toPublic(p store.Profile) publicProfile {
	return publicProfile{Handle: p.Handle, DisplayName: p.DisplayName, Bio: p.Bio, AvatarColor: p.AvatarColor}
}

func (h *handlers) getMyProfile(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	p, err := h.users.GetProfile(r.Context(), s.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
			return
		}
		h.serverError(w, r, err, "getMyProfile failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": p})
}

func (h *handlers) patchMyProfile(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())

	var body profilePatchReq
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Normalize, then validate every field so the form can show all
	// problems at once (same shape as item attribute errors).
	fields := map[string]string{}
	patch := store.ProfilePatch{
		AcceptFriendRequests: body.AcceptFriendRequests,
		AcceptFollowRequests: body.AcceptFollowRequests,
	}
	if body.Handle != nil {
		v := profile.NormalizeHandle(*body.Handle)
		if msg := profile.ValidateHandle(v); msg != "" {
			fields["handle"] = msg
		}
		patch.Handle = &v
	}
	if body.DisplayName != nil {
		v := profile.NormalizeDisplayName(*body.DisplayName)
		if msg := profile.ValidateDisplayName(v); msg != "" {
			fields["display_name"] = msg
		}
		patch.DisplayName = &v
	}
	if body.Bio != nil {
		v := profile.NormalizeBio(*body.Bio)
		if msg := profile.ValidateBio(v); msg != "" {
			fields["bio"] = msg
		}
		patch.Bio = &v
	}
	if body.CollectionVisibility != nil {
		v, ok := privacy.ParseVisibility(*body.CollectionVisibility)
		if !ok {
			fields["collection_visibility"] = msgVisibility
		}
		patch.CollectionVisibility = &v
	}
	if len(fields) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": msgFixFields, "fields": fields})
		return
	}

	p, err := h.users.UpdateProfile(r.Context(), s.UserID, patch)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrHandleTaken):
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": profile.MsgHandleTaken, "fields": map[string]string{"handle": profile.MsgHandleTaken},
			})
		case errors.Is(err, store.ErrInvalidHandle), errors.Is(err, store.ErrInvalidProfile):
			// The handler's validation should have caught it; keep the shape.
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": msgFixFields, "fields": map[string]string{}})
		case errors.Is(err, store.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
		default:
			h.serverError(w, r, err, "patchMyProfile failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": p})
}

// handleAvailability answers the settings form's live check. It's signed-in
// only and rate limited per user, to make bulk enumeration impractical, and
// your own current handle counts as available.
func (h *handlers) handleAvailability(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if !h.handleChecks.Allow(s.UserID) {
		w.Header().Set("Retry-After", strconv.Itoa(int(handleCheckInterval.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": msgTooManyChecks})
		return
	}
	raw := r.URL.Query().Get("handle")
	if raw == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "handle is required"})
		return
	}
	handle := profile.NormalizeHandle(raw)
	if msg := profile.ValidateHandle(handle); msg != "" {
		writeJSON(w, http.StatusOK, map[string]any{"handle": handle, "available": false, "message": msg})
		return
	}
	ok, err := h.users.HandleAvailable(r.Context(), handle, s.UserID)
	if err != nil {
		h.serverError(w, r, err, "handleAvailability failed")
		return
	}
	msg := ""
	if !ok {
		msg = profile.MsgHandleTaken
	}
	writeJSON(w, http.StatusOK, map[string]any{"handle": handle, "available": ok, "message": msg})
}

// deleteMe deletes the signed-in user's account and everything they own,
// then clears the session. The body must repeat the user's handle
// ({"confirm_handle": "..."}): the web asks them to type it, and requiring it
// here means a stray or scripted DELETE can't wipe an account by itself.
func (h *handlers) deleteMe(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	var body struct {
		ConfirmHandle string `json:"confirm_handle"`
	}
	if err := decodeJSONBody(r.Body, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": msgConfirm, "fields": map[string]string{"confirm_handle": msgConfirm},
		})
		return
	}
	p, err := h.users.GetProfile(r.Context(), s.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.clearSession(w)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.serverError(w, r, err, "deleteMe: profile lookup failed")
		return
	}
	if profile.NormalizeHandle(body.ConfirmHandle) != p.Handle {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": msgConfirm, "fields": map[string]string{"confirm_handle": msgConfirm},
		})
		return
	}
	if err := h.users.Delete(r.Context(), s.UserID); err != nil && !errors.Is(err, store.ErrNotFound) {
		h.serverError(w, r, err, "deleteMe failed")
		return
	}
	h.logger.Info("account deleted", "user_id", s.UserID)
	h.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) clearSession(w http.ResponseWriter) {
	if h.sessions != nil {
		h.sessions.Clear(w)
	}
}

// viewAccess is what a viewer may see of an owner, decided by package
// privacy. found=false means "answer 404": the profile doesn't exist or is
// hidden from this viewer, and the two must look identical.
type viewAccess struct {
	owner           store.Profile
	rel             privacy.Relationship
	found           bool
	seesCollections bool
}

// resolveView is the single path every /users/{handle} endpoint uses to
// apply the privacy rule.
func (h *handlers) resolveView(ctx context.Context, handle string) (viewAccess, error) {
	owner, err := h.users.GetProfileByHandle(ctx, handle)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return viewAccess{}, nil
		}
		return viewAccess{}, err
	}
	viewerID := ""
	if s, ok := auth.FromContext(ctx); ok {
		viewerID = s.UserID
	}
	looked, err := h.relationships.Between(ctx, viewerID, owner.UserID)
	if err != nil {
		return viewAccess{}, err
	}
	rel := privacy.RelationshipFor(viewerID, owner.UserID, looked)
	if !privacy.CanViewProfile(owner.CollectionVisibility, rel) {
		return viewAccess{}, nil
	}
	return viewAccess{
		owner:           owner,
		rel:             rel,
		found:           true,
		seesCollections: privacy.CanViewCollections(owner.CollectionVisibility, rel),
	}, nil
}

// publicCategory is what a collection viewer sees of its category: whether
// it's on the public list, never the review status of someone else's
// category (private vs rejected); the owner gets the status too.
func publicCategory(cat store.Category, isSelf bool) map[string]any {
	out := map[string]any{
		"slug":             cat.Slug,
		"name":             cat.Name,
		"attribute_schema": cat.AttributeSchema,
		"public":           cat.Status == store.CategoryApproved,
	}
	if isSelf {
		out["status"] = cat.Status
	}
	return out
}

func writeProfileNotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "profile not found"})
}

// getPublicProfile is GET /users/{handle}: the profile card, plus the
// owner's collections when the privacy rule allows. Signed-in is optional.
func (h *handlers) getPublicProfile(w http.ResponseWriter, r *http.Request) {
	v, err := h.resolveView(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		h.serverError(w, r, err, "getPublicProfile failed")
		return
	}
	if !v.found {
		writeProfileNotFound(w)
		return
	}
	collections := []store.Collection{}
	if v.seesCollections {
		collections, err = h.collections.ListByUser(r.Context(), v.owner.UserID)
		if err != nil {
			h.serverError(w, r, err, "getPublicProfile: collections failed")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"profile":               toPublic(v.owner),
		"is_self":               v.rel == privacy.Self,
		"collection_visibility": v.owner.CollectionVisibility,
		"collections_visible":   v.seesCollections,
		"collections":           collections,
	})
}

// getPublicCollection is GET /users/{handle}/collections/{id}: one of the
// owner's collections with its items, read-only. 404 unless the viewer may
// see the owner's collections and the collection belongs to that owner.
func (h *handlers) getPublicCollection(w http.ResponseWriter, r *http.Request) {
	v, err := h.resolveView(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		h.serverError(w, r, err, "getPublicCollection failed")
		return
	}
	if !v.found || !v.seesCollections {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
		return
	}
	id := chi.URLParam(r, "id")
	coll, err := h.collections.GetByIDForUser(r.Context(), id, v.owner.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "getPublicCollection: collection failed")
		return
	}
	items, err := h.items.ListInCollectionForUser(r.Context(), id, v.owner.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "collection not found"})
			return
		}
		h.serverError(w, r, err, "getPublicCollection: items failed")
		return
	}
	// The privacy rule let this viewer see the collection, so they may see
	// its category too, even an unapproved one the owner built (ADR 0013).
	// Its schema gives the items' attributes their labels.
	cat, err := h.categories.GetByID(r.Context(), coll.CategoryID)
	if err != nil {
		h.serverError(w, r, err, "getPublicCollection: category failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"owner":      toPublic(v.owner),
		"is_self":    v.rel == privacy.Self,
		"collection": coll,
		"category":   publicCategory(cat, v.rel == privacy.Self),
		"items":      items,
	})
}
