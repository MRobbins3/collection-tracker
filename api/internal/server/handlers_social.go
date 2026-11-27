package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/privacy"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
	"github.com/MRobbins3/collection-tracker/api/internal/textnorm"
)

// SocialStore is friends and followers (ADR 0014). store.Relationships
// implements it. Every method takes user ids; handlers resolve handles.
type SocialStore interface {
	State(ctx context.Context, viewerID, otherID string) (store.RelationshipState, error)
	SendFriendRequest(ctx context.Context, from, to string) (store.FriendStatus, error)
	CancelFriendRequest(ctx context.Context, from, to string) error
	AcceptFriendRequest(ctx context.Context, me, from string) error
	DeclineFriendRequest(ctx context.Context, me, from string) error
	Unfriend(ctx context.Context, me, other string) error
	RequestFollow(ctx context.Context, follower, followee string) (store.FollowStatus, error)
	Unfollow(ctx context.Context, follower, followee string) error
	ApproveFollower(ctx context.Context, me, follower string) error
	RemoveFollower(ctx context.Context, me, follower string) error
	DenyFollower(ctx context.Context, me, follower string) error
	ListFriends(ctx context.Context, me string) ([]store.Person, error)
	ListFollowers(ctx context.Context, me string) ([]store.Person, error)
	ListFollowing(ctx context.Context, me string) ([]store.Person, error)
	ListRequests(ctx context.Context, me string) (store.Requests, error)
	IncomingRequestCount(ctx context.Context, me string) (int, error)
	SearchPeople(ctx context.Context, viewerID, q string, limit int) ([]store.Person, error)
}

// Sending friend and follow requests is rate limited per user, so nobody
// can spam requests (or ask, cancel, and ask again) at someone: 30 quick
// ones, then one every 10 seconds. People search gets its own budget,
// generous enough for typing, tight enough to make scraping slow.
const (
	requestBurst        = 30
	requestInterval     = 10 * time.Second
	searchBurst         = 60
	searchInterval      = time.Second
	peopleSearchLimit   = 20
	peopleSearchMinLen  = 2
	msgTooManyRequests  = "You’re sending requests too quickly. Wait a moment and try again."
	msgTooManySearches  = "You’re searching too quickly. Wait a moment and try again."
	msgNotYourself      = "That’s you."
	msgNotAcceptFriends = "@%s isn’t accepting friend requests."
	msgNotAcceptFollows = "@%s isn’t accepting follow requests."
)

// viewerRelationship is the relationship block of GET /users/{handle} for
// a signed-in viewer who isn't the owner: where things stand, plus whether
// the owner takes new requests, so the page shows the right buttons. The
// toggles reveal nothing a refused request wouldn't.
type viewerRelationship struct {
	store.RelationshipState
	AcceptsFriendRequests bool `json:"accepts_friend_requests"`
	AcceptsFollowRequests bool `json:"accepts_follow_requests"`
}

// relationshipFor builds the block, or nil for anonymous viewers, the
// owner, or when no SocialStore is wired.
func (h *handlers) relationshipFor(ctx context.Context, v viewAccess) (*viewerRelationship, error) {
	s, ok := auth.FromContext(ctx)
	if !ok || h.social == nil || v.rel == privacy.Self {
		return nil, nil
	}
	st, err := h.social.State(ctx, s.UserID, v.owner.UserID)
	if err != nil {
		return nil, err
	}
	return &viewerRelationship{
		RelationshipState:     st,
		AcceptsFriendRequests: v.owner.AcceptFriendRequests,
		AcceptsFollowRequests: v.owner.AcceptFollowRequests,
	}, nil
}

// requestTarget resolves the profile a new request goes to. It goes
// through resolveView, so a profile the sender can't see is 404 exactly
// like an unknown handle. ok=false means a response was written.
func (h *handlers) requestTarget(w http.ResponseWriter, r *http.Request) (viewAccess, bool) {
	s, _ := auth.FromContext(r.Context())
	if !h.requestLimits.Allow(s.UserID) {
		w.Header().Set("Retry-After", strconv.Itoa(int(requestInterval.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": msgTooManyRequests})
		return viewAccess{}, false
	}
	v, err := h.resolveView(r.Context(), chi.URLParam(r, "handle"))
	if err != nil {
		h.serverError(w, r, err, "request target lookup failed")
		return viewAccess{}, false
	}
	if !v.found {
		writeProfileNotFound(w)
		return viewAccess{}, false
	}
	if v.rel == privacy.Self {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msgNotYourself})
		return viewAccess{}, false
	}
	return v, true
}

// counterpartID resolves the other person in an answer or removal. It
// skips the privacy rule on purpose: you must be able to accept a request
// from, or remove, someone whose profile you can't see. found=false for an
// unknown handle; callers answer exactly as they would for "nothing to
// do", so these endpoints reveal nothing about who exists.
func (h *handlers) counterpartID(ctx context.Context, handle string) (string, bool, error) {
	p, err := h.users.GetProfileByHandle(ctx, handle)
	if errors.Is(err, store.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return p.UserID, true, nil
}

func (h *handlers) writeRelationship(w http.ResponseWriter, r *http.Request, v viewAccess) {
	rel, err := h.relationshipFor(r.Context(), v)
	if err != nil {
		h.serverError(w, r, err, "relationship state failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"relationship": rel})
}

func notAccepting(w http.ResponseWriter, format, handle string) {
	writeJSON(w, http.StatusForbidden, map[string]string{"error": fmt.Sprintf(format, handle)})
}

// POST /users/{handle}/friend-request
func (h *handlers) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	v, ok := h.requestTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.social.SendFriendRequest(r.Context(), s.UserID, v.owner.UserID); err != nil {
		switch {
		case errors.Is(err, store.ErrNotAccepting):
			notAccepting(w, msgNotAcceptFriends, v.owner.Handle)
		case errors.Is(err, store.ErrNotFound):
			writeProfileNotFound(w)
		default:
			h.serverError(w, r, err, "sendFriendRequest failed")
		}
		return
	}
	h.writeRelationship(w, r, v)
}

// POST /users/{handle}/follow
func (h *handlers) requestFollow(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	v, ok := h.requestTarget(w, r)
	if !ok {
		return
	}
	if _, err := h.social.RequestFollow(r.Context(), s.UserID, v.owner.UserID); err != nil {
		switch {
		case errors.Is(err, store.ErrNotAccepting):
			notAccepting(w, msgNotAcceptFollows, v.owner.Handle)
		case errors.Is(err, store.ErrNotFound):
			writeProfileNotFound(w)
		default:
			h.serverError(w, r, err, "requestFollow failed")
		}
		return
	}
	h.writeRelationship(w, r, v)
}

// removal handles every idempotent "make it go away" endpoint: cancel,
// decline, unfriend, unfollow, deny, remove follower. It answers 204 even
// for an unknown handle, so it can't be used to probe who exists.
func (h *handlers) removal(op func(ctx context.Context, me, other string) error, what string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, _ := auth.FromContext(r.Context())
		otherID, found, err := h.counterpartID(r.Context(), chi.URLParam(r, "handle"))
		if err != nil {
			h.serverError(w, r, err, what+" lookup failed")
			return
		}
		if found && otherID != s.UserID {
			if err := op(r.Context(), s.UserID, otherID); err != nil {
				h.serverError(w, r, err, what+" failed")
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// approval handles accept (friend) and approve (follow): 204 when done or
// already done, 404 when there's no such request to you (including an
// unknown handle), so only the recipient can ever say yes.
func (h *handlers) approval(op func(ctx context.Context, me, other string) error, what string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, _ := auth.FromContext(r.Context())
		otherID, found, err := h.counterpartID(r.Context(), chi.URLParam(r, "handle"))
		if err != nil {
			h.serverError(w, r, err, what+" lookup failed")
			return
		}
		if found && otherID != s.UserID {
			err = op(r.Context(), s.UserID, otherID)
		} else {
			err = store.ErrNotFound
		}
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "request not found"})
				return
			}
			h.serverError(w, r, err, what+" failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *handlers) listPeople(op func(ctx context.Context, me string) ([]store.Person, error), what string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, _ := auth.FromContext(r.Context())
		people, err := op(r.Context(), s.UserID)
		if err != nil {
			h.serverError(w, r, err, what+" failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"people": nonNil(people)})
	}
}

func nonNil(ps []store.Person) []store.Person {
	if ps == nil {
		return []store.Person{}
	}
	return ps
}

// GET /me/requests
func (h *handlers) listMyRequests(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	reqs, err := h.social.ListRequests(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "listMyRequests failed")
		return
	}
	reqs.FriendIncoming = nonNil(reqs.FriendIncoming)
	reqs.FriendOutgoing = nonNil(reqs.FriendOutgoing)
	reqs.FollowIncoming = nonNil(reqs.FollowIncoming)
	reqs.FollowOutgoing = nonNil(reqs.FollowOutgoing)
	writeJSON(w, http.StatusOK, reqs)
}

// GET /me/requests/count: the header badge.
func (h *handlers) countMyRequests(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	n, err := h.social.IncomingRequestCount(r.Context(), s.UserID)
	if err != nil {
		h.serverError(w, r, err, "countMyRequests failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"incoming": n})
}

// GET /people?q=: find people by handle or display name among the profiles
// the viewer may see. Signed in only and rate limited, so it can't be used
// to list every member.
func (h *handlers) searchPeople(w http.ResponseWriter, r *http.Request) {
	s, _ := auth.FromContext(r.Context())
	if !h.searchLimits.Allow(s.UserID) {
		w.Header().Set("Retry-After", strconv.Itoa(int(searchInterval.Seconds())))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": msgTooManySearches})
		return
	}
	q := r.URL.Query().Get("q")
	if len([]rune(strings.TrimPrefix(textnorm.Clean(q), "@"))) < peopleSearchMinLen {
		writeJSON(w, http.StatusOK, map[string]any{"people": []store.Person{}})
		return
	}
	people, err := h.social.SearchPeople(r.Context(), s.UserID, q, peopleSearchLimit)
	if err != nil {
		h.serverError(w, r, err, "searchPeople failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"people": nonNil(people)})
}
