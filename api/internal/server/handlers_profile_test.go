package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/privacy"
	"github.com/MRobbins3/collection-tracker/api/internal/profile"
	"github.com/MRobbins3/collection-tracker/api/internal/server"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

// ---- fakes ---------------------------------------------------------------

// fakeProfiles backs the profile methods of fakeUsers. Handle rules mirror
// the real store (normalize, then case-insensitive uniqueness).
type fakeProfiles struct {
	mu      sync.Mutex
	byID    map[string]store.Profile
	deleted []string
}

func newFakeProfiles(ps ...store.Profile) *fakeProfiles {
	f := &fakeProfiles{byID: map[string]store.Profile{}}
	for _, p := range ps {
		f.byID[p.UserID] = p
	}
	return f
}

func (f *fakeUsers) profileStore() *fakeProfiles {
	if f.profiles == nil {
		f.profiles = newFakeProfiles()
	}
	return f.profiles
}

func (f *fakeUsers) GetProfile(_ context.Context, userID string) (store.Profile, error) {
	p := f.profileStore()
	p.mu.Lock()
	defer p.mu.Unlock()
	got, ok := p.byID[userID]
	if !ok {
		return store.Profile{}, store.ErrNotFound
	}
	return got, nil
}

func (f *fakeUsers) GetProfileByHandle(_ context.Context, handle string) (store.Profile, error) {
	p := f.profileStore()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, got := range p.byID {
		if got.Handle == profile.NormalizeHandle(handle) {
			return got, nil
		}
	}
	return store.Profile{}, store.ErrNotFound
}

func (f *fakeUsers) HandleAvailable(_ context.Context, handle, userID string) (bool, error) {
	p := f.profileStore()
	p.mu.Lock()
	defer p.mu.Unlock()
	h := profile.NormalizeHandle(handle)
	if profile.ValidateHandle(h) != "" {
		return false, nil
	}
	for id, got := range p.byID {
		if got.Handle == h && id != userID {
			return false, nil
		}
	}
	return true, nil
}

func (f *fakeUsers) UpdateProfile(_ context.Context, userID string, patch store.ProfilePatch) (store.Profile, error) {
	p := f.profileStore()
	p.mu.Lock()
	defer p.mu.Unlock()
	cur, ok := p.byID[userID]
	if !ok {
		return store.Profile{}, store.ErrNotFound
	}
	if patch.Handle != nil {
		h := profile.NormalizeHandle(*patch.Handle)
		for id, other := range p.byID {
			if other.Handle == h && id != userID {
				return store.Profile{}, store.ErrHandleTaken
			}
		}
		cur.Handle = h
	}
	if patch.DisplayName != nil {
		cur.DisplayName = *patch.DisplayName
	}
	if patch.Bio != nil {
		cur.Bio = *patch.Bio
	}
	if patch.CollectionVisibility != nil {
		cur.CollectionVisibility = *patch.CollectionVisibility
	}
	if patch.AcceptFriendRequests != nil {
		cur.AcceptFriendRequests = *patch.AcceptFriendRequests
	}
	if patch.AcceptFollowRequests != nil {
		cur.AcceptFollowRequests = *patch.AcceptFollowRequests
	}
	p.byID[userID] = cur
	return cur, nil
}

func (f *fakeUsers) Delete(_ context.Context, userID string) error {
	p := f.profileStore()
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.byID[userID]; !ok {
		return store.ErrNotFound
	}
	delete(p.byID, userID)
	p.deleted = append(p.deleted, userID)
	return nil
}

// fakeRelationships returns a fixed relationship for one viewer→owner pair,
// standing in for the friends/followers lookup that arrives later.
type fakeRelationships struct {
	viewer, owner string
	rel           privacy.Relationship
}

func (f fakeRelationships) Between(_ context.Context, viewerID, ownerID string) (privacy.Relationship, error) {
	if viewerID == f.viewer && ownerID == f.owner {
		return privacy.RelationshipFor(viewerID, ownerID, f.rel), nil
	}
	return privacy.RelationshipFor(viewerID, ownerID, privacy.None), nil
}

// ---- fixtures ------------------------------------------------------------

func mkProfile(id, handle, name string, vis privacy.Visibility) store.Profile {
	return store.Profile{
		UserID: id, Handle: handle, DisplayName: name, Bio: name + "'s bio",
		AvatarColor: "teal", CollectionVisibility: vis,
		AcceptFriendRequests: true, AcceptFollowRequests: true,
	}
}

type profileFixture struct {
	users *fakeUsers
	colls *fakeCollections
	items *fakeItems
	rels  server.RelationshipStore
	// collection ids by owner handle
	collIDs map[string]string
	cats    *fakeCategories
	// social, when set, backs both Relationships and Social.
	social *fakeSocial
}

func newProfileFixture(t *testing.T) *profileFixture {
	t.Helper()
	users := &fakeUsers{profiles: newFakeProfiles(
		mkProfile("u-liam", "liam", "Liam Johnson", privacy.Public),
		mkProfile("u-emma", "emma", "Emma Garcia", privacy.Private),
		mkProfile("u-noah", "noah", "Noah Williams", privacy.FriendsOnly),
		mkProfile("u-amelia", "amelia", "Amelia Brown", privacy.FollowersAndFriends),
		mkProfile("u-olivia", "olivia", "Olivia Martinez", privacy.Public),
	)}
	colls := newFakeCollections()
	items := newFakeItems()
	ctx := context.Background()
	ids := map[string]string{}
	for _, owner := range []string{"liam", "emma", "noah", "amelia"} {
		c, err := colls.Create(ctx, "u-"+owner, "coins", owner+"'s coins")
		require.NoError(t, err)
		_, err = items.Create(ctx, c.ID, "u-"+owner, store.ItemInput{Name: owner + "'s penny", Quantity: 1})
		require.NoError(t, err)
		ids[owner] = c.ID
	}
	return &profileFixture{users: users, colls: colls, items: items, collIDs: ids, cats: &fakeCategories{}}
}

func (f *profileFixture) deps() server.Deps {
	d := server.Deps{
		Categories:    f.cats,
		Collections:   f.colls,
		Items:         f.items,
		Users:         f.users,
		Relationships: f.rels,
	}
	if f.social != nil {
		d.Relationships = f.social
		d.Social = f.social
	}
	return d
}

// router returns a router where every request is made as viewerID; ""
// means anonymous.
func (f *profileFixture) router(t *testing.T, viewerID string) http.Handler {
	t.Helper()
	if viewerID != "" {
		return newAuthenticatedRouterWithDeps(t, f.deps(), viewerID)
	}
	deps := f.deps()
	deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	deps.DBPinger = fakePinger{}
	deps.Sessions = auth.NewManager([]byte("01234567890123456789012345678901"), false)
	deps.CORSOrigins = []string{"http://localhost:3000"}
	return server.NewRouter(deps)
}

type publicProfileResp struct {
	Profile              map[string]any     `json:"profile"`
	IsSelf               bool               `json:"is_self"`
	CollectionVisibility string             `json:"collection_visibility"`
	CollectionsVisible   bool               `json:"collections_visible"`
	Collections          []store.Collection `json:"collections"`
}

func decode[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out), rr.Body.String())
	return out
}

// ---- /me/profile ---------------------------------------------------------

func TestMyProfileRoutesRequireAuth(t *testing.T) {
	f := newProfileFixture(t)
	h := f.router(t, "")
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/me/profile"},
		{http.MethodPatch, "/me/profile"},
		{http.MethodGet, "/me/handle-availability?handle=abc"},
		{http.MethodDelete, "/me"},
	} {
		rr := doJSON(t, h, c.method, c.path, map[string]any{})
		require.Equal(t, http.StatusUnauthorized, rr.Code, c.path)
	}
}

func TestGetMyProfile(t *testing.T) {
	f := newProfileFixture(t)
	rr := doJSON(t, f.router(t, "u-emma"), http.MethodGet, "/me/profile", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	got := decode[struct{ Profile store.Profile }](t, rr)
	require.Equal(t, "emma", got.Profile.Handle)
	require.Equal(t, privacy.Private, got.Profile.CollectionVisibility)
	require.True(t, got.Profile.AcceptFriendRequests)
}

func TestPatchMyProfileNormalizesAndSaves(t *testing.T) {
	f := newProfileFixture(t)
	rr := doJSON(t, f.router(t, "u-emma"), http.MethodPatch, "/me/profile", map[string]any{
		"handle":                 "  @Emma_G ",
		"display_name":           "  Emma   García ",
		"bio":                    " Coins,\n stamps. ",
		"collection_visibility":  "public",
		"accept_follow_requests": false,
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got := decode[struct{ Profile store.Profile }](t, rr)
	require.Equal(t, "emma_g", got.Profile.Handle)
	require.Equal(t, "Emma García", got.Profile.DisplayName)
	require.Equal(t, "Coins, stamps.", got.Profile.Bio)
	require.Equal(t, privacy.Public, got.Profile.CollectionVisibility)
	require.False(t, got.Profile.AcceptFollowRequests)
	require.True(t, got.Profile.AcceptFriendRequests, "omitted fields are untouched")
}

func TestPatchMyProfileValidationIsPerField(t *testing.T) {
	f := newProfileFixture(t)
	h := f.router(t, "u-emma")

	cases := []struct {
		name  string
		body  map[string]any
		field string
		msg   string
	}{
		{"handle chars", map[string]any{"handle": "emma garcia"}, "handle", profile.MsgHandleChars},
		{"handle length", map[string]any{"handle": "em"}, "handle", profile.MsgHandleLength},
		{"handle start", map[string]any{"handle": "2emma"}, "handle", profile.MsgHandleStart},
		{"handle reserved", map[string]any{"handle": "Settings"}, "handle", profile.MsgHandleReserved},
		{"empty name", map[string]any{"display_name": "   "}, "display_name", profile.MsgDisplayNameLength},
		{"long bio", map[string]any{"bio": strings.Repeat("a", 161)}, "bio", profile.MsgBioLength},
		{"bad visibility", map[string]any{"collection_visibility": "everyone"}, "collection_visibility", "Choose who can see your collections."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rr := doJSON(t, h, http.MethodPatch, "/me/profile", c.body)
			require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
			got := decode[struct {
				Error  string
				Fields map[string]string
			}](t, rr)
			require.NotEmpty(t, got.Error)
			require.Equal(t, map[string]string{c.field: c.msg}, got.Fields)
		})
	}

	t.Run("every bad field is reported at once, nothing is saved", func(t *testing.T) {
		rr := doJSON(t, h, http.MethodPatch, "/me/profile", map[string]any{
			"handle": "x", "display_name": "", "bio": "ok",
		})
		require.Equal(t, http.StatusBadRequest, rr.Code)
		got := decode[struct{ Fields map[string]string }](t, rr)
		require.Len(t, got.Fields, 2)
		p, err := f.users.GetProfile(context.Background(), "u-emma")
		require.NoError(t, err)
		require.Equal(t, "Emma Garcia's bio", p.Bio)
	})

	t.Run("unknown fields are rejected", func(t *testing.T) {
		rr := doJSON(t, h, http.MethodPatch, "/me/profile", map[string]any{"email": "x@y"})
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})
}

func TestPatchMyProfileHandleTakenIs409WithFieldMessage(t *testing.T) {
	f := newProfileFixture(t)
	rr := doJSON(t, f.router(t, "u-emma"), http.MethodPatch, "/me/profile", map[string]any{"handle": "LIAM"})
	require.Equal(t, http.StatusConflict, rr.Code)
	got := decode[struct{ Fields map[string]string }](t, rr)
	require.Equal(t, profile.MsgHandleTaken, got.Fields["handle"])
}

func TestHandleAvailability(t *testing.T) {
	f := newProfileFixture(t)
	h := f.router(t, "u-emma")
	cases := []struct {
		q, handle string
		available bool
		msg       string
	}{
		{"brand_new", "brand_new", true, ""},
		{"@Brand_New", "brand_new", true, ""},
		{"emma", "emma", true, ""}, // your own
		{"Liam", "liam", false, profile.MsgHandleTaken},
		{"admin", "admin", false, profile.MsgHandleReserved},
		{"no", "no", false, profile.MsgHandleLength},
		{"has space", "has space", false, profile.MsgHandleChars},
	}
	for _, c := range cases {
		rr := doJSON(t, h, http.MethodGet, "/me/handle-availability?handle="+strings.ReplaceAll(c.q, " ", "%20"), nil)
		require.Equal(t, http.StatusOK, rr.Code, c.q)
		got := decode[struct {
			Handle    string
			Available bool
			Message   string
		}](t, rr)
		require.Equal(t, c.handle, got.Handle, c.q)
		require.Equal(t, c.available, got.Available, c.q)
		require.Equal(t, c.msg, got.Message, c.q)
	}

	rr := doJSON(t, h, http.MethodGet, "/me/handle-availability", nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandleAvailabilityIsRateLimitedPerUser(t *testing.T) {
	f := newProfileFixture(t)
	base := f.router(t, "") // one router, so one limiter, for both users
	sessions := auth.NewManager([]byte("01234567890123456789012345678901"), false)
	emma := authedWrap(base, sessions, "u-emma")
	var last *httptest.ResponseRecorder
	for i := 0; i < 21; i++ {
		last = doJSON(t, emma, http.MethodGet, "/me/handle-availability?handle=probe_"+itoa(i+1), nil)
		if i < 20 {
			require.Equal(t, http.StatusOK, last.Code, "check %d", i+1)
		}
	}
	require.Equal(t, http.StatusTooManyRequests, last.Code)
	require.NotEmpty(t, last.Header().Get("Retry-After"))
	require.Contains(t, decode[struct{ Error string }](t, last).Error, "too quickly")

	// The budget is per user.
	rr := doJSON(t, authedWrap(base, sessions, "u-liam"), http.MethodGet, "/me/handle-availability?handle=probe", nil)
	require.Equal(t, http.StatusOK, rr.Code)
}

// ---- DELETE /me ----------------------------------------------------------

func TestDeleteMyAccount(t *testing.T) {
	f := newProfileFixture(t)
	h := f.router(t, "u-emma")

	rr := doJSON(t, h, http.MethodDelete, "/me", map[string]any{"confirm_handle": "liam"})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	got := decode[struct{ Fields map[string]string }](t, rr)
	require.Contains(t, got.Fields, "confirm_handle")
	require.Empty(t, f.users.profiles.deleted)

	rr = doJSON(t, h, http.MethodDelete, "/me", nil)
	require.Equal(t, http.StatusBadRequest, rr.Code, "a confirmation body is required")

	rr = doJSON(t, h, http.MethodDelete, "/me", map[string]any{"confirm_handle": " @EMMA "})
	require.Equal(t, http.StatusNoContent, rr.Code, rr.Body.String())
	require.Equal(t, []string{"u-emma"}, f.users.profiles.deleted)

	var cleared bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "ct_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	require.True(t, cleared, "the session cookie is cleared")
}

// ---- GET /users/{handle} -------------------------------------------------

func TestPublicProfileVisibilityMatrix(t *testing.T) {
	type want struct {
		status             int
		collectionsVisible bool
	}
	cases := []struct {
		name, viewer, handle string
		want                 want
	}{
		{"public, anonymous", "", "liam", want{200, true}},
		{"public, signed-in stranger", "u-emma", "liam", want{200, true}},
		{"public, self", "u-liam", "liam", want{200, true}},
		{"private, anonymous → 404", "", "emma", want{404, false}},
		{"private, signed-in stranger → 404", "u-liam", "emma", want{404, false}},
		{"private, self", "u-emma", "emma", want{200, true}},
		{"friends only, anonymous", "", "noah", want{200, false}},
		{"friends only, stranger", "u-liam", "noah", want{200, false}},
		{"friends only, self", "u-noah", "noah", want{200, true}},
		{"followers & friends, stranger", "u-liam", "amelia", want{200, false}},
		{"unknown handle → 404", "u-liam", "nobody", want{404, false}},
		{"handle lookup is case-insensitive", "", "LIAM", want{200, true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newProfileFixture(t)
			rr := doJSON(t, f.router(t, c.viewer), http.MethodGet, "/users/"+c.handle, nil)
			require.Equal(t, c.want.status, rr.Code, rr.Body.String())
			if c.want.status == http.StatusNotFound {
				// Hidden and missing look identical, so existence doesn't leak.
				require.JSONEq(t, `{"error":"profile not found"}`, rr.Body.String())
				return
			}
			got := decode[publicProfileResp](t, rr)
			require.Equal(t, c.want.collectionsVisible, got.CollectionsVisible)
			if c.want.collectionsVisible {
				require.Len(t, got.Collections, 1)
			} else {
				require.NotNil(t, got.Collections)
				require.Empty(t, got.Collections, "hidden collections are never sent")
			}
		})
	}
}

func TestPublicProfileShowsOnlyPublicFieldsToOthers(t *testing.T) {
	f := newProfileFixture(t)
	rr := doJSON(t, f.router(t, "u-emma"), http.MethodGet, "/users/liam", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	got := decode[publicProfileResp](t, rr)
	require.False(t, got.IsSelf)
	require.Equal(t, "public", got.CollectionVisibility)
	require.Equal(t, "liam", got.Profile["handle"])
	require.Equal(t, "Liam Johnson", got.Profile["display_name"])
	require.Equal(t, "Liam Johnson's bio", got.Profile["bio"])
	require.Equal(t, "teal", got.Profile["avatar_color"])
	for _, private := range []string{"email", "accept_friend_requests", "accept_follow_requests", "id", "user_id"} {
		require.NotContains(t, got.Profile, private)
	}
	require.NotContains(t, rr.Body.String(), "u-liam", "internal user ids stay internal")

	rr = doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/liam", nil)
	require.True(t, decode[publicProfileResp](t, rr).IsSelf)
}

// The follower/friend levels work as soon as the relationship lookup
// reports them; nothing in the handler changes.
func TestPublicProfileHonorsRelationshipLookup(t *testing.T) {
	f := newProfileFixture(t)
	f.rels = fakeRelationships{viewer: "u-liam", owner: "u-amelia", rel: privacy.Follower}
	got := decode[publicProfileResp](t, doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/amelia", nil))
	require.True(t, got.CollectionsVisible, "a follower sees followers & friends collections")

	f.rels = fakeRelationships{viewer: "u-liam", owner: "u-noah", rel: privacy.Follower}
	got = decode[publicProfileResp](t, doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/noah", nil))
	require.False(t, got.CollectionsVisible, "a follower is not a friend")

	f.rels = fakeRelationships{viewer: "u-liam", owner: "u-emma", rel: privacy.Friend}
	rr := doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/emma", nil)
	require.Equal(t, http.StatusOK, rr.Code, "a friend can find a private profile")
	require.False(t, decode[publicProfileResp](t, rr).CollectionsVisible, "but not its collections")
}

// ---- GET /users/{handle}/collections/{id} --------------------------------

func TestPublicCollection(t *testing.T) {
	f := newProfileFixture(t)

	rr := doJSON(t, f.router(t, ""), http.MethodGet, "/users/liam/collections/"+f.collIDs["liam"], nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got := decode[struct {
		Owner      map[string]any
		IsSelf     bool `json:"is_self"`
		Collection store.Collection
		Category   map[string]any
		Items      []store.Item
	}](t, rr)
	require.Equal(t, "liam", got.Owner["handle"])
	// The category comes with it (ADR 0013): a viewer allowed to see the
	// collection sees its category's labels even if it isn't public.
	require.NotEmpty(t, got.Category["name"])
	require.Contains(t, got.Category, "attribute_schema")
	require.Equal(t, "liam's coins", got.Collection.Name)
	require.Len(t, got.Items, 1)
	require.False(t, got.IsSelf)

	notFound := []struct{ name, viewer, path string }{
		{"friends-only owner, stranger", "u-liam", "/users/noah/collections/" + f.collIDs["noah"]},
		{"private owner, anonymous", "", "/users/emma/collections/" + f.collIDs["emma"]},
		{"someone else's collection under a public handle", "", "/users/liam/collections/" + f.collIDs["noah"]},
		{"unknown collection", "", "/users/liam/collections/col-999"},
		{"unknown handle", "", "/users/nobody/collections/" + f.collIDs["liam"]},
	}
	for _, c := range notFound {
		rr := doJSON(t, f.router(t, c.viewer), http.MethodGet, c.path, nil)
		require.Equal(t, http.StatusNotFound, rr.Code, c.name)
	}

	rr = doJSON(t, f.router(t, "u-noah"), http.MethodGet, "/users/noah/collections/"+f.collIDs["noah"], nil)
	require.Equal(t, http.StatusOK, rr.Code, "owners always see their own")
}

// The collection's category comes along even when its owner built it and
// it isn't public (ADR 0013), but viewers only learn whether it's public,
// never its review status; blocked viewers get the plain 404 with no
// category data at all.
func TestPublicCollectionCategoryHidesReviewStatus(t *testing.T) {
	f := newProfileFixture(t)
	f.cats.byIDStatus = store.CategoryRejected

	rr := doJSON(t, f.router(t, "u-olivia"), http.MethodGet, "/users/liam/collections/"+f.collIDs["liam"], nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	cat := decode[struct {
		Category map[string]any
	}](t, rr).Category
	require.Equal(t, false, cat["public"])
	require.NotContains(t, cat, "status")
	require.NotContains(t, rr.Body.String(), "rejected")

	rr = doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/liam/collections/"+f.collIDs["liam"], nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "rejected", decode[struct {
		Category map[string]any
	}](t, rr).Category["status"], "the owner sees their own category's status")

	blocked := doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/noah/collections/"+f.collIDs["noah"], nil)
	unknown := doJSON(t, f.router(t, "u-liam"), http.MethodGet, "/users/noah/collections/col-999", nil)
	require.Equal(t, http.StatusNotFound, blocked.Code)
	require.Equal(t, unknown.Body.String(), blocked.Body.String(), "same 404 as a collection that doesn't exist")
	require.NotContains(t, blocked.Body.String(), "category")
	require.NotContains(t, blocked.Body.String(), "Lego")
}

// After DELETE /me, a session cookie on another device still decodes, but
// its user is gone: signed-in routes answer 401 and clear the cookie
// instead of failing on foreign keys.
func TestSessionForDeletedUserIs401AndClearsCookie(t *testing.T) {
	deps := server.Deps{
		Categories:  &fakeCategories{},
		Collections: newFakeCollections(),
		Items:       newFakeItems(),
		Users:       &fakeUsers{errByID: store.ErrNotFound},
	}
	h := newAuthenticatedRouterWithDeps(t, deps, "u-deleted")
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/me/collections"},
		{http.MethodPost, "/me/collections"},
		{http.MethodGet, "/me/profile"},
	} {
		rr := doJSON(t, h, c.method, c.path, map[string]any{"name": "x", "category_slug": "coins"})
		require.Equal(t, http.StatusUnauthorized, rr.Code, c.path)
		var cleared bool
		for _, ck := range rr.Result().Cookies() {
			if ck.Name == "ct_session" && ck.MaxAge < 0 {
				cleared = true
			}
		}
		require.True(t, cleared, c.path)
	}
}
