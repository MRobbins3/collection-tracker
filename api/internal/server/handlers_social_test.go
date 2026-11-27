package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// ---- fake ----------------------------------------------------------------

type fakeFriendship struct {
	requestedBy string
	accepted    bool
}

// fakeSocial is an in-memory stand-in for store.Relationships with the same
// rules (the real one is covered by store integration tests). It reads the
// "accept … requests" toggles and visibility from the fake profiles.
type fakeSocial struct {
	mu       sync.Mutex
	profiles *fakeProfiles
	friends  map[[2]string]*fakeFriendship // ordered pair
	follows  map[[2]string]bool            // follower, followee → approved
}

func newFakeSocial(p *fakeProfiles) *fakeSocial {
	return &fakeSocial{profiles: p, friends: map[[2]string]*fakeFriendship{}, follows: map[[2]string]bool{}}
}

func pair(x, y string) [2]string {
	if y < x {
		return [2]string{y, x}
	}
	return [2]string{x, y}
}

func (f *fakeSocial) profile(id string) (store.Profile, bool) {
	f.profiles.mu.Lock()
	defer f.profiles.mu.Unlock()
	p, ok := f.profiles.byID[id]
	return p, ok
}

func (f *fakeSocial) Between(_ context.Context, viewerID, ownerID string) (privacy.Relationship, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return privacy.RelationshipFor(viewerID, ownerID, f.betweenLocked(viewerID, ownerID)), nil
}

func (f *fakeSocial) betweenLocked(viewerID, ownerID string) privacy.Relationship {
	fr := f.friends[pair(viewerID, ownerID)]
	switch {
	case fr != nil && fr.accepted:
		return privacy.Friend
	case f.follows[[2]string{viewerID, ownerID}]:
		return privacy.Follower
	case fr != nil && fr.requestedBy == ownerID:
		return privacy.Pending
	}
	if approved, ok := f.follows[[2]string{ownerID, viewerID}]; ok && !approved {
		return privacy.Pending
	}
	return privacy.None
}

func (f *fakeSocial) stateLocked(viewerID, otherID string) store.RelationshipState {
	st := store.RelationshipState{Friend: store.FriendNone, Following: store.FollowNone, FollowsYou: store.FollowNone}
	if fr := f.friends[pair(viewerID, otherID)]; fr != nil {
		switch {
		case fr.accepted:
			st.Friend = store.Friends
		case fr.requestedBy == viewerID:
			st.Friend = store.FriendRequestSent
		default:
			st.Friend = store.FriendRequestReceived
		}
	}
	follow := func(k [2]string) store.FollowStatus {
		approved, ok := f.follows[k]
		switch {
		case !ok:
			return store.FollowNone
		case approved:
			return store.FollowFollowing
		}
		return store.FollowRequested
	}
	st.Following = follow([2]string{viewerID, otherID})
	st.FollowsYou = follow([2]string{otherID, viewerID})
	return st
}

func (f *fakeSocial) State(_ context.Context, viewerID, otherID string) (store.RelationshipState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stateLocked(viewerID, otherID), nil
}

func (f *fakeSocial) SendFriendRequest(_ context.Context, from, to string) (store.FriendStatus, error) {
	if from == to {
		return "", store.ErrSelfRelationship
	}
	target, ok := f.profile(to)
	if !ok {
		return "", store.ErrNotFound
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	k := pair(from, to)
	if fr := f.friends[k]; fr != nil {
		if !fr.accepted && fr.requestedBy == to {
			fr.accepted = true
		}
		return f.stateLocked(from, to).Friend, nil
	}
	if !target.AcceptFriendRequests {
		return "", store.ErrNotAccepting
	}
	f.friends[k] = &fakeFriendship{requestedBy: from}
	return store.FriendRequestSent, nil
}

func (f *fakeSocial) CancelFriendRequest(_ context.Context, from, to string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := pair(from, to)
	if fr := f.friends[k]; fr != nil && !fr.accepted && fr.requestedBy == from {
		delete(f.friends, k)
	}
	return nil
}

func (f *fakeSocial) DeclineFriendRequest(ctx context.Context, me, from string) error {
	return f.CancelFriendRequest(ctx, from, me)
}

func (f *fakeSocial) AcceptFriendRequest(_ context.Context, me, from string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fr := f.friends[pair(me, from)]
	switch {
	case fr == nil:
		return store.ErrNotFound
	case fr.accepted:
		return nil
	case fr.requestedBy != from:
		return store.ErrNotFound
	}
	fr.accepted = true
	return nil
}

func (f *fakeSocial) Unfriend(_ context.Context, me, other string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := pair(me, other)
	if fr := f.friends[k]; fr != nil && fr.accepted {
		delete(f.friends, k)
	}
	return nil
}

func (f *fakeSocial) RequestFollow(_ context.Context, follower, followee string) (store.FollowStatus, error) {
	if follower == followee {
		return "", store.ErrSelfRelationship
	}
	target, ok := f.profile(followee)
	if !ok {
		return "", store.ErrNotFound
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	k := [2]string{follower, followee}
	if _, exists := f.follows[k]; !exists {
		if !target.AcceptFollowRequests {
			return "", store.ErrNotAccepting
		}
		f.follows[k] = false
	}
	return f.stateLocked(follower, followee).Following, nil
}

func (f *fakeSocial) Unfollow(_ context.Context, follower, followee string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.follows, [2]string{follower, followee})
	return nil
}

func (f *fakeSocial) RemoveFollower(ctx context.Context, me, follower string) error {
	return f.Unfollow(ctx, follower, me)
}

func (f *fakeSocial) DenyFollower(_ context.Context, me, follower string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := [2]string{follower, me}
	if approved, ok := f.follows[k]; ok && !approved {
		delete(f.follows, k)
	}
	return nil
}

func (f *fakeSocial) ApproveFollower(_ context.Context, me, follower string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := [2]string{follower, me}
	if _, ok := f.follows[k]; !ok {
		return store.ErrNotFound
	}
	f.follows[k] = true
	return nil
}

func (f *fakeSocial) person(id string) store.Person {
	p, _ := f.profile(id)
	return store.Person{UserID: id, Handle: p.Handle, DisplayName: p.DisplayName, AvatarColor: p.AvatarColor, Since: time.Unix(0, 0)}
}

func sortPeople(ps []store.Person) []store.Person {
	sort.Slice(ps, func(i, j int) bool { return ps[i].Handle < ps[j].Handle })
	return ps
}

func (f *fakeSocial) ListFriends(_ context.Context, me string) ([]store.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.Person{}
	for k, fr := range f.friends {
		if fr.accepted && (k[0] == me || k[1] == me) {
			other := k[0]
			if other == me {
				other = k[1]
			}
			out = append(out, f.person(other))
		}
	}
	return sortPeople(out), nil
}

func (f *fakeSocial) listFollows(me string, incoming, approved bool) []store.Person {
	out := []store.Person{}
	for k, ok := range f.follows {
		mine, theirs := k[0], k[1]
		if incoming {
			mine, theirs = theirs, mine
		}
		if mine == me && ok == approved {
			out = append(out, f.person(theirs))
		}
	}
	return sortPeople(out)
}

func (f *fakeSocial) ListFollowers(_ context.Context, me string) ([]store.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listFollows(me, true, true), nil
}

func (f *fakeSocial) ListFollowing(_ context.Context, me string) ([]store.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listFollows(me, false, true), nil
}

func (f *fakeSocial) ListRequests(_ context.Context, me string) (store.Requests, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := store.Requests{FriendIncoming: []store.Person{}, FriendOutgoing: []store.Person{}}
	for k, fr := range f.friends {
		if fr.accepted || (k[0] != me && k[1] != me) {
			continue
		}
		other := k[0]
		if other == me {
			other = k[1]
		}
		if fr.requestedBy == me {
			out.FriendOutgoing = append(out.FriendOutgoing, f.person(other))
		} else {
			out.FriendIncoming = append(out.FriendIncoming, f.person(other))
		}
	}
	out.FollowIncoming = f.listFollows(me, true, false)
	out.FollowOutgoing = f.listFollows(me, false, false)
	return out, nil
}

func (f *fakeSocial) IncomingRequestCount(ctx context.Context, me string) (int, error) {
	r, _ := f.ListRequests(ctx, me)
	return len(r.FriendIncoming) + len(r.FollowIncoming), nil
}

func (f *fakeSocial) SearchPeople(_ context.Context, viewerID, q string, limit int) ([]store.Person, error) {
	f.profiles.mu.Lock()
	all := make([]store.Profile, 0, len(f.profiles.byID))
	for _, p := range f.profiles.byID {
		all = append(all, p)
	}
	f.profiles.mu.Unlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	q = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(q), "@"))
	out := []store.Person{}
	for _, p := range all {
		if p.UserID == viewerID {
			continue
		}
		if !strings.HasPrefix(p.Handle, q) && !strings.Contains(strings.ToLower(p.DisplayName), q) {
			continue
		}
		if !privacy.CanViewProfile(p.CollectionVisibility, f.betweenLocked(viewerID, p.UserID)) {
			continue
		}
		found := f.person(p.UserID)
		found.Since = time.Time{}
		out = append(out, found)
	}
	out = sortPeople(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---- helpers -------------------------------------------------------------

// socialRouters builds one router (one fake, one set of limiters) and hands
// out a signed-in view of it per user id.
func socialRouters(t *testing.T) (*profileFixture, func(userID string) http.Handler) {
	t.Helper()
	f := newProfileFixture(t)
	f.social = newFakeSocial(f.users.profiles)
	base := f.router(t, "")
	sessions := auth.NewManager([]byte("01234567890123456789012345678901"), false)
	return f, func(userID string) http.Handler {
		if userID == "" {
			return base
		}
		return authedWrap(base, sessions, userID)
	}
}

type relResp struct {
	Relationship *struct {
		Friend                string `json:"friend"`
		Following             string `json:"following"`
		FollowsYou            string `json:"follows_you"`
		AcceptsFriendRequests bool   `json:"accepts_friend_requests"`
		AcceptsFollowRequests bool   `json:"accepts_follow_requests"`
	} `json:"relationship"`
	CollectionsVisible bool `json:"collections_visible"`
}

func profileAs(t *testing.T, h http.Handler, handle string) relResp {
	t.Helper()
	rr := doJSON(t, h, http.MethodGet, "/users/"+handle, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	return decode[relResp](t, rr)
}

type peopleResp struct {
	People []map[string]any `json:"people"`
}

func peopleHandles(t *testing.T, rr *httptest.ResponseRecorder) []string {
	t.Helper()
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	out := []string{}
	for _, p := range decode[peopleResp](t, rr).People {
		out = append(out, p["handle"].(string))
	}
	return out
}

// ---- tests ---------------------------------------------------------------

func TestSocialRoutesRequireAuth(t *testing.T) {
	_, as := socialRouters(t)
	h := as("")
	for _, c := range []struct{ method, path string }{
		{http.MethodPost, "/users/noah/friend-request"},
		{http.MethodDelete, "/users/noah/friend-request"},
		{http.MethodPost, "/users/noah/follow"},
		{http.MethodDelete, "/users/noah/follow"},
		{http.MethodPost, "/me/friend-requests/noah/accept"},
		{http.MethodPost, "/me/friend-requests/noah/decline"},
		{http.MethodDelete, "/me/friends/noah"},
		{http.MethodPost, "/me/follow-requests/noah/approve"},
		{http.MethodPost, "/me/follow-requests/noah/deny"},
		{http.MethodDelete, "/me/followers/noah"},
		{http.MethodGet, "/me/friends"},
		{http.MethodGet, "/me/followers"},
		{http.MethodGet, "/me/following"},
		{http.MethodGet, "/me/requests"},
		{http.MethodGet, "/me/requests/count"},
		{http.MethodGet, "/people?q=noah"},
	} {
		rr := doJSON(t, h, c.method, c.path, map[string]any{})
		require.Equal(t, http.StatusUnauthorized, rr.Code, "%s %s", c.method, c.path)
	}
}

// Liam asks Noah (Friends only) to be friends; once Noah accepts, Liam sees
// Noah's collections.
func TestFriendRequestUnlocksFriendsOnlyCollections(t *testing.T) {
	_, as := socialRouters(t)
	liam, noah := as("u-liam"), as("u-noah")

	before := profileAs(t, liam, "noah")
	require.False(t, before.CollectionsVisible)
	require.Equal(t, "none", before.Relationship.Friend)
	require.True(t, before.Relationship.AcceptsFriendRequests)

	rr := doJSON(t, liam, http.MethodPost, "/users/noah/friend-request", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "request_sent", decode[relResp](t, rr).Relationship.Friend)
	require.False(t, profileAs(t, liam, "noah").CollectionsVisible, "asking unlocks nothing")
	require.Equal(t, "request_received", profileAs(t, noah, "liam").Relationship.Friend)

	rr = doJSON(t, liam, http.MethodPost, "/me/friend-requests/noah/accept", nil)
	require.Equal(t, http.StatusNotFound, rr.Code, "the sender can't accept their own request")
	rr = doJSON(t, as("u-amelia"), http.MethodPost, "/me/friend-requests/liam/accept", nil)
	require.Equal(t, http.StatusNotFound, rr.Code, "nor can anyone else")

	rr = doJSON(t, noah, http.MethodPost, "/me/friend-requests/liam/accept", nil)
	require.Equal(t, http.StatusNoContent, rr.Code, rr.Body.String())
	after := profileAs(t, liam, "noah")
	require.True(t, after.CollectionsVisible)
	require.Equal(t, "friends", after.Relationship.Friend)
	require.Equal(t, []string{"noah"}, peopleHandles(t, doJSON(t, liam, http.MethodGet, "/me/friends", nil)))

	rr = doJSON(t, noah, http.MethodDelete, "/me/friends/liam", nil)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.False(t, profileAs(t, liam, "noah").CollectionsVisible, "unfriending locks it again")
}

func TestFriendRequestCancelAndDeclineAreIdempotent(t *testing.T) {
	_, as := socialRouters(t)
	liam, noah := as("u-liam"), as("u-noah")
	require.Equal(t, http.StatusOK, doJSON(t, liam, http.MethodPost, "/users/noah/friend-request", nil).Code)
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusNoContent, doJSON(t, liam, http.MethodDelete, "/users/noah/friend-request", nil).Code)
	}
	require.Equal(t, "none", profileAs(t, noah, "liam").Relationship.Friend)

	require.Equal(t, http.StatusOK, doJSON(t, liam, http.MethodPost, "/users/noah/friend-request", nil).Code)
	for i := 0; i < 2; i++ {
		require.Equal(t, http.StatusNoContent, doJSON(t, noah, http.MethodPost, "/me/friend-requests/liam/decline", nil).Code)
	}
	require.Equal(t, "none", profileAs(t, liam, "noah").Relationship.Friend)

	// Unknown handles answer the same as known ones, so removals reveal nothing.
	require.Equal(t, http.StatusNoContent, doJSON(t, liam, http.MethodDelete, "/users/nobody_here/friend-request", nil).Code)
	require.Equal(t, http.StatusNoContent, doJSON(t, liam, http.MethodDelete, "/me/friends/nobody_here", nil).Code)
	require.Equal(t, http.StatusNotFound, doJSON(t, liam, http.MethodPost, "/me/friend-requests/nobody_here/accept", nil).Code)
}

// A stranger can't reach a Private profile with a request: it answers
// exactly like a handle that doesn't exist.
func TestRequestsToHiddenProfilesAre404(t *testing.T) {
	_, as := socialRouters(t)
	liam := as("u-liam")
	for _, path := range []string{"/users/emma/friend-request", "/users/emma/follow"} {
		hidden := doJSON(t, liam, http.MethodPost, path, nil)
		missing := doJSON(t, liam, http.MethodPost, strings.Replace(path, "emma", "nobody_here", 1), nil)
		require.Equal(t, http.StatusNotFound, hidden.Code, path)
		require.Equal(t, missing.Code, hidden.Code)
		require.JSONEq(t, missing.Body.String(), hidden.Body.String())
	}
}

func TestRequestsRespectAcceptToggles(t *testing.T) {
	f, as := socialRouters(t)
	off := false
	_, err := f.users.UpdateProfile(context.Background(), "u-noah", store.ProfilePatch{
		AcceptFriendRequests: &off, AcceptFollowRequests: &off,
	})
	require.NoError(t, err)
	liam := as("u-liam")

	view := profileAs(t, liam, "noah")
	require.False(t, view.Relationship.AcceptsFriendRequests)
	require.False(t, view.Relationship.AcceptsFollowRequests)

	rr := doJSON(t, liam, http.MethodPost, "/users/noah/friend-request", nil)
	require.Equal(t, http.StatusForbidden, rr.Code)
	require.JSONEq(t, `{"error":"@noah isn’t accepting friend requests."}`, rr.Body.String())
	rr = doJSON(t, liam, http.MethodPost, "/users/noah/follow", nil)
	require.Equal(t, http.StatusForbidden, rr.Code)
	require.JSONEq(t, `{"error":"@noah isn’t accepting follow requests."}`, rr.Body.String())
}

func TestNoRequestsToYourself(t *testing.T) {
	_, as := socialRouters(t)
	liam := as("u-liam")
	require.Equal(t, http.StatusBadRequest, doJSON(t, liam, http.MethodPost, "/users/liam/friend-request", nil).Code)
	require.Equal(t, http.StatusBadRequest, doJSON(t, liam, http.MethodPost, "/users/LIAM/follow", nil).Code)
}

// Liam asks to follow Amelia (Followers & friends): approval unlocks her
// collections, removing him locks them again, and a denied request unlocks
// nothing.
func TestFollowApproveDenyAndRemove(t *testing.T) {
	_, as := socialRouters(t)
	liam, amelia := as("u-liam"), as("u-amelia")

	rr := doJSON(t, liam, http.MethodPost, "/users/amelia/follow", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "requested", decode[relResp](t, rr).Relationship.Following)
	require.False(t, profileAs(t, liam, "amelia").CollectionsVisible)
	require.Equal(t, "requested", profileAs(t, amelia, "liam").Relationship.FollowsYou)

	require.Equal(t, http.StatusNotFound, doJSON(t, liam, http.MethodPost, "/me/follow-requests/amelia/approve", nil).Code,
		"only the person being followed approves")
	require.Equal(t, http.StatusNoContent, doJSON(t, amelia, http.MethodPost, "/me/follow-requests/liam/approve", nil).Code)
	require.True(t, profileAs(t, liam, "amelia").CollectionsVisible)
	require.Equal(t, []string{"liam"}, peopleHandles(t, doJSON(t, amelia, http.MethodGet, "/me/followers", nil)))
	require.Equal(t, []string{"amelia"}, peopleHandles(t, doJSON(t, liam, http.MethodGet, "/me/following", nil)))
	require.Empty(t, peopleHandles(t, doJSON(t, amelia, http.MethodGet, "/me/friends", nil)), "followers aren't friends")

	require.Equal(t, http.StatusNoContent, doJSON(t, amelia, http.MethodDelete, "/me/followers/liam", nil).Code)
	require.False(t, profileAs(t, liam, "amelia").CollectionsVisible, "removing a follower revokes access")

	require.Equal(t, http.StatusOK, doJSON(t, liam, http.MethodPost, "/users/amelia/follow", nil).Code)
	require.Equal(t, http.StatusNoContent, doJSON(t, amelia, http.MethodPost, "/me/follow-requests/liam/deny", nil).Code)
	require.Equal(t, "none", profileAs(t, liam, "amelia").Relationship.Following)

	// Deny is only for pending requests; an approved follower stays until removed.
	require.Equal(t, http.StatusOK, doJSON(t, liam, http.MethodPost, "/users/amelia/follow", nil).Code)
	require.Equal(t, http.StatusNoContent, doJSON(t, amelia, http.MethodPost, "/me/follow-requests/liam/approve", nil).Code)
	require.Equal(t, http.StatusNoContent, doJSON(t, amelia, http.MethodPost, "/me/follow-requests/liam/deny", nil).Code)
	require.Equal(t, "following", profileAs(t, liam, "amelia").Relationship.Following)
	require.Equal(t, http.StatusNoContent, doJSON(t, liam, http.MethodDelete, "/users/amelia/follow", nil).Code)

	require.Equal(t, http.StatusOK, doJSON(t, liam, http.MethodPost, "/users/amelia/follow", nil).Code)
	require.Equal(t, http.StatusNoContent, doJSON(t, liam, http.MethodDelete, "/users/amelia/follow", nil).Code)
	require.Equal(t, "none", profileAs(t, liam, "amelia").Relationship.Following)
}

// Emma (Private) asks Liam to be friends: Liam can now see her card to
// decide, but not her collections, and can accept from his requests list.
func TestRecipientSeesAPrivateRequesterCard(t *testing.T) {
	_, as := socialRouters(t)
	emma, liam := as("u-emma"), as("u-liam")
	require.Equal(t, http.StatusNotFound, doJSON(t, liam, http.MethodGet, "/users/emma", nil).Code)

	require.Equal(t, http.StatusOK, doJSON(t, emma, http.MethodPost, "/users/liam/friend-request", nil).Code)
	view := profileAs(t, liam, "emma")
	require.False(t, view.CollectionsVisible)
	require.Equal(t, "request_received", view.Relationship.Friend)

	rr := doJSON(t, liam, http.MethodGet, "/me/requests", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	got := decode[struct {
		FriendIncoming []map[string]any `json:"friend_incoming"`
		FriendOutgoing []map[string]any `json:"friend_outgoing"`
		FollowIncoming []map[string]any `json:"follow_incoming"`
		FollowOutgoing []map[string]any `json:"follow_outgoing"`
	}](t, rr)
	require.Len(t, got.FriendIncoming, 1)
	require.Equal(t, "emma", got.FriendIncoming[0]["handle"])
	require.NotContains(t, got.FriendIncoming[0], "user_id", "no internal ids")
	require.NotContains(t, got.FriendIncoming[0], "email")
	require.NotNil(t, got.FollowIncoming)

	rr = doJSON(t, liam, http.MethodGet, "/me/requests/count", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"incoming":1}`, rr.Body.String())
}

func TestProfileRelationshipIsNullForAnonymousAndSelf(t *testing.T) {
	_, as := socialRouters(t)
	require.Nil(t, profileAs(t, as(""), "liam").Relationship)
	require.Nil(t, profileAs(t, as("u-liam"), "liam").Relationship)
}

func TestPeopleSearchNeverReturnsHiddenProfiles(t *testing.T) {
	_, as := socialRouters(t)
	liam := as("u-liam")
	// Every fixture name contains an "a"; Emma is Private.
	got := peopleHandles(t, doJSON(t, liam, http.MethodGet, "/people?q=a", nil))
	require.Empty(t, got, "queries under two characters return nothing")

	got = peopleHandles(t, doJSON(t, liam, http.MethodGet, "/people?q=em", nil))
	require.NotContains(t, got, "emma")

	got = peopleHandles(t, doJSON(t, liam, http.MethodGet, "/people?q=%40noah", nil))
	require.Equal(t, []string{"noah"}, got)

	rr := doJSON(t, liam, http.MethodGet, "/people?q=noah", nil)
	p := decode[peopleResp](t, rr).People[0]
	require.ElementsMatch(t, []string{"handle", "display_name", "avatar_color"}, keys(p),
		"no ids, emails, or sign-up times")
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestSendingRequestsIsRateLimited(t *testing.T) {
	_, as := socialRouters(t)
	liam := as("u-liam")
	var last *httptest.ResponseRecorder
	for i := 0; i < 31; i++ {
		// Asking and cancelling over and over is exactly what the limit is for.
		last = doJSON(t, liam, http.MethodPost, "/users/noah/follow", nil)
		if i < 30 {
			require.Equal(t, http.StatusOK, last.Code, "request %d", i+1)
		}
		doJSON(t, liam, http.MethodDelete, "/users/noah/follow", nil)
	}
	require.Equal(t, http.StatusTooManyRequests, last.Code)
	require.NotEmpty(t, last.Header().Get("Retry-After"))
}
