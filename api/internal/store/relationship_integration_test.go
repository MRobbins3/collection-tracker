//go:build integration

package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

type relFixture struct {
	users *store.Users
	rels  *store.Relationships
	ids   map[string]string // by handle
}

// newRelFixture makes one user per name; each gets the handle of its
// lowercased name (e.g. "Alice" → @alice).
func newRelFixture(t *testing.T, names ...string) (*relFixture, context.Context) {
	t.Helper()
	pool, cleanup := provisionSeededDB(t, t.Name())
	t.Cleanup(cleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	f := &relFixture{users: store.NewUsers(pool), rels: store.NewRelationships(pool), ids: map[string]string{}}
	for _, n := range names {
		u, err := f.users.UpsertByGoogleSub(ctx, "g-"+n, n+"@example.com", n)
		require.NoError(t, err)
		f.ids[u.Handle] = u.ID
	}
	return f, ctx
}

func (f *relFixture) between(t *testing.T, ctx context.Context, viewer, owner string) privacy.Relationship {
	t.Helper()
	rel, err := f.rels.Between(ctx, f.ids[viewer], f.ids[owner])
	require.NoError(t, err)
	return rel
}

func (f *relFixture) state(t *testing.T, ctx context.Context, viewer, owner string) store.RelationshipState {
	t.Helper()
	s, err := f.rels.State(ctx, f.ids[viewer], f.ids[owner])
	require.NoError(t, err)
	return s
}

func handles(ps []store.Person) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Handle)
	}
	return out
}

func TestFriendRequestLifecycle(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]

	st, err := f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, store.FriendRequestSent, st)
	require.Equal(t, privacy.None, f.between(t, ctx, "alice", "bob"), "asking unlocks nothing")
	require.Equal(t, privacy.Pending, f.between(t, ctx, "bob", "alice"), "the recipient may see who's asking")
	require.Equal(t, store.FriendRequestReceived, f.state(t, ctx, "bob", "alice").Friend)

	again, err := f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, store.FriendRequestSent, again, "sending twice is idempotent")

	require.ErrorIs(t, f.rels.AcceptFriendRequest(ctx, a, b), store.ErrNotFound,
		"only the recipient can accept")

	require.NoError(t, f.rels.AcceptFriendRequest(ctx, b, a))
	require.NoError(t, f.rels.AcceptFriendRequest(ctx, b, a), "accepting twice is idempotent")
	require.Equal(t, privacy.Friend, f.between(t, ctx, "alice", "bob"))
	require.Equal(t, privacy.Friend, f.between(t, ctx, "bob", "alice"))
	require.Equal(t, store.Friends, f.state(t, ctx, "alice", "bob").Friend)

	friends, err := f.rels.ListFriends(ctx, a)
	require.NoError(t, err)
	require.Equal(t, []string{"bob"}, handles(friends))

	require.NoError(t, f.rels.Unfriend(ctx, b, a), "either friend can unfriend")
	require.NoError(t, f.rels.Unfriend(ctx, b, a), "unfriending twice is idempotent")
	require.Equal(t, privacy.None, f.between(t, ctx, "alice", "bob"))
}

func TestFriendRequestCancelAndDecline(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]

	_, err := f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	require.NoError(t, f.rels.DeclineFriendRequest(ctx, a, b), "the sender can't decline their own request")
	require.Equal(t, store.FriendRequestReceived, f.state(t, ctx, "bob", "alice").Friend)
	require.NoError(t, f.rels.CancelFriendRequest(ctx, b, a), "the recipient can't cancel it either")
	require.Equal(t, store.FriendRequestReceived, f.state(t, ctx, "bob", "alice").Friend)

	require.NoError(t, f.rels.CancelFriendRequest(ctx, a, b))
	require.Equal(t, store.FriendNone, f.state(t, ctx, "bob", "alice").Friend)

	_, err = f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err, "a cancelled request can be sent again")
	require.NoError(t, f.rels.DeclineFriendRequest(ctx, b, a))
	require.Equal(t, store.FriendNone, f.state(t, ctx, "alice", "bob").Friend)
	require.ErrorIs(t, f.rels.AcceptFriendRequest(ctx, b, a), store.ErrNotFound, "nothing left to accept")

	_, err = f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	require.NoError(t, f.rels.Unfriend(ctx, b, a), "unfriend leaves a pending request alone")
	require.Equal(t, store.FriendRequestReceived, f.state(t, ctx, "bob", "alice").Friend)
}

// Asking someone who already asked you makes you friends, whatever their
// "accept friend requests" setting: they asked first.
func TestMutualFriendRequestsBecomeFriends(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]
	_, err := f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	_, err = f.users.UpdateProfile(ctx, a, store.ProfilePatch{AcceptFriendRequests: boolp(false)})
	require.NoError(t, err)

	st, err := f.rels.SendFriendRequest(ctx, b, a)
	require.NoError(t, err)
	require.Equal(t, store.Friends, st)
	require.Equal(t, privacy.Friend, f.between(t, ctx, "alice", "bob"))
}

// Two people asking each other at the same moment end up as friends, never
// with two crossing requests (the ordered-pair primary key serializes them).
func TestConcurrentMutualFriendRequests(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]
	for round := 0; round < 10; round++ {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); _, errs[0] = f.rels.SendFriendRequest(ctx, a, b) }()
		go func() { defer wg.Done(); _, errs[1] = f.rels.SendFriendRequest(ctx, b, a) }()
		wg.Wait()
		require.NoError(t, errs[0])
		require.NoError(t, errs[1])
		require.Equal(t, privacy.Friend, f.between(t, ctx, "alice", "bob"), "round %d", round)
		require.NoError(t, f.rels.Unfriend(ctx, a, b))
	}
}

func TestRequestsRespectAcceptSettings(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]
	_, err := f.users.UpdateProfile(ctx, b, store.ProfilePatch{
		AcceptFriendRequests: boolp(false), AcceptFollowRequests: boolp(false),
	})
	require.NoError(t, err)

	_, err = f.rels.SendFriendRequest(ctx, a, b)
	require.ErrorIs(t, err, store.ErrNotAccepting)
	_, err = f.rels.RequestFollow(ctx, a, b)
	require.ErrorIs(t, err, store.ErrNotAccepting)
	require.Equal(t, store.RelationshipState{
		Friend: store.FriendNone, Following: store.FollowNone, FollowsYou: store.FollowNone,
	}, f.state(t, ctx, "alice", "bob"))

	// Turning requests off later doesn't undo what's already there; asking
	// again just reports it.
	_, err = f.users.UpdateProfile(ctx, b, store.ProfilePatch{
		AcceptFriendRequests: boolp(true), AcceptFollowRequests: boolp(true),
	})
	require.NoError(t, err)
	_, err = f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	_, err = f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	_, err = f.users.UpdateProfile(ctx, b, store.ProfilePatch{
		AcceptFriendRequests: boolp(false), AcceptFollowRequests: boolp(false),
	})
	require.NoError(t, err)
	st, err := f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, store.FriendRequestSent, st)
	fs, err := f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, store.FollowRequested, fs)
}

func TestNoRelationshipWithYourself(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice")
	a := f.ids["alice"]
	_, err := f.rels.SendFriendRequest(ctx, a, a)
	require.ErrorIs(t, err, store.ErrSelfRelationship)
	_, err = f.rels.RequestFollow(ctx, a, a)
	require.ErrorIs(t, err, store.ErrSelfRelationship)
}

func TestFollowLifecycle(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]

	st, err := f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, store.FollowRequested, st)
	st, err = f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, store.FollowRequested, st, "asking twice is idempotent")
	require.Equal(t, privacy.None, f.between(t, ctx, "alice", "bob"), "a pending follow unlocks nothing")
	require.Equal(t, privacy.Pending, f.between(t, ctx, "bob", "alice"))
	require.Equal(t, store.FollowRequested, f.state(t, ctx, "bob", "alice").FollowsYou)

	require.ErrorIs(t, f.rels.ApproveFollower(ctx, a, b), store.ErrNotFound, "only the followee approves")
	require.NoError(t, f.rels.ApproveFollower(ctx, b, a))
	require.NoError(t, f.rels.ApproveFollower(ctx, b, a), "approving twice is idempotent")
	require.Equal(t, privacy.Follower, f.between(t, ctx, "alice", "bob"))
	require.Equal(t, privacy.None, f.between(t, ctx, "bob", "alice"), "following is one-way")
	require.Equal(t, store.FollowFollowing, f.state(t, ctx, "alice", "bob").Following)

	followers, err := f.rels.ListFollowers(ctx, b)
	require.NoError(t, err)
	require.Equal(t, []string{"alice"}, handles(followers))
	following, err := f.rels.ListFollowing(ctx, a)
	require.NoError(t, err)
	require.Equal(t, []string{"bob"}, handles(following))
	friends, err := f.rels.ListFriends(ctx, b)
	require.NoError(t, err)
	require.Empty(t, friends, "followers are not friends")

	require.NoError(t, f.rels.RemoveFollower(ctx, b, a), "the followee can remove a follower")
	require.Equal(t, privacy.None, f.between(t, ctx, "alice", "bob"))

	_, err = f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	require.NoError(t, f.rels.RemoveFollower(ctx, b, a), "and deny a pending request the same way")
	require.Equal(t, store.FollowNone, f.state(t, ctx, "alice", "bob").Following)

	_, err = f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	require.NoError(t, f.rels.Unfollow(ctx, a, b), "the follower can cancel a pending request")
	require.Equal(t, store.FollowNone, f.state(t, ctx, "alice", "bob").Following)
}

func TestFriendOutranksFollower(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob")
	a, b := f.ids["alice"], f.ids["bob"]
	_, err := f.rels.RequestFollow(ctx, a, b)
	require.NoError(t, err)
	require.NoError(t, f.rels.ApproveFollower(ctx, b, a))
	_, err = f.rels.SendFriendRequest(ctx, a, b)
	require.NoError(t, err)
	require.Equal(t, privacy.Follower, f.between(t, ctx, "alice", "bob"))
	require.NoError(t, f.rels.AcceptFriendRequest(ctx, b, a))
	require.Equal(t, privacy.Friend, f.between(t, ctx, "alice", "bob"))
}

func TestListRequests(t *testing.T) {
	f, ctx := newRelFixture(t, "Alice", "Bob", "Carol", "Dave")
	a, b, c, d := f.ids["alice"], f.ids["bob"], f.ids["carol"], f.ids["dave"]
	_, err := f.rels.SendFriendRequest(ctx, b, a)
	require.NoError(t, err)
	_, err = f.rels.RequestFollow(ctx, c, a)
	require.NoError(t, err)
	_, err = f.rels.SendFriendRequest(ctx, a, d)
	require.NoError(t, err)
	_, err = f.rels.RequestFollow(ctx, a, c)
	require.NoError(t, err)

	reqs, err := f.rels.ListRequests(ctx, a)
	require.NoError(t, err)
	require.Equal(t, []string{"bob"}, handles(reqs.FriendIncoming))
	require.Equal(t, []string{"dave"}, handles(reqs.FriendOutgoing))
	require.Equal(t, []string{"carol"}, handles(reqs.FollowIncoming))
	require.Equal(t, []string{"carol"}, handles(reqs.FollowOutgoing))

	n, err := f.rels.IncomingRequestCount(ctx, a)
	require.NoError(t, err)
	require.Equal(t, 2, n)

	other, err := f.rels.ListRequests(ctx, b)
	require.NoError(t, err)
	require.Empty(t, other.FriendIncoming, "a pending request is visible only to its two people")
	require.Equal(t, []string{"alice"}, handles(other.FriendOutgoing))
}

func TestSearchPeopleHidesPrivateProfiles(t *testing.T) {
	f, ctx := newRelFixture(t, "Viewer", "Sam Public", "Sam Private", "Sam Friend", "Sam Asker", "Sam Asked")
	v := f.ids["viewer"]
	setVis := func(handle string, vis privacy.Visibility) {
		_, err := f.users.UpdateProfile(ctx, f.ids[handle], store.ProfilePatch{CollectionVisibility: &vis})
		require.NoError(t, err)
	}
	setVis("sam_public", privacy.Public)
	for _, h := range []string{"sam_private", "sam_friend", "sam_asker", "sam_asked"} {
		setVis(h, privacy.Private)
	}
	_, err := f.rels.SendFriendRequest(ctx, f.ids["sam_friend"], v)
	require.NoError(t, err)
	require.NoError(t, f.rels.AcceptFriendRequest(ctx, v, f.ids["sam_friend"]))
	_, err = f.rels.RequestFollow(ctx, f.ids["sam_asker"], v)
	require.NoError(t, err)
	// The viewer asking a Private user doesn't reveal them.
	_, err = f.rels.SendFriendRequest(ctx, v, f.ids["sam_asked"])
	require.NoError(t, err)

	got, err := f.rels.SearchPeople(ctx, v, "sam", 20)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"sam_public", "sam_friend", "sam_asker"}, handles(got))

	// Each result must agree with the privacy rule.
	for _, p := range got {
		rel, err := f.rels.Between(ctx, v, p.UserID)
		require.NoError(t, err)
		prof, err := f.users.GetProfile(ctx, p.UserID)
		require.NoError(t, err)
		require.True(t, privacy.CanViewProfile(prof.CollectionVisibility, rel), p.Handle)
	}

	self, err := f.rels.SearchPeople(ctx, v, "viewer", 20)
	require.NoError(t, err)
	require.Empty(t, self, "you don't find yourself")

	byName, err := f.rels.SearchPeople(ctx, v, "  SAM   pub ", 20)
	require.NoError(t, err)
	require.Equal(t, []string{"sam_public"}, handles(byName), "display names match ignoring case and spaces")

	byHandle, err := f.rels.SearchPeople(ctx, v, "@sam_pu", 20)
	require.NoError(t, err)
	require.Equal(t, []string{"sam_public"}, handles(byHandle))

	wild, err := f.rels.SearchPeople(ctx, v, "%", 20)
	require.NoError(t, err)
	require.Empty(t, wild, "LIKE wildcards are literal")
}

func TestSearchPeopleRanksHandleMatchesFirst(t *testing.T) {
	f, ctx := newRelFixture(t, "Viewer", "Zed Collector", "Collector")
	pub := privacy.Public
	for _, h := range []string{"zed_collector", "collector"} {
		_, err := f.users.UpdateProfile(ctx, f.ids[h], store.ProfilePatch{CollectionVisibility: &pub})
		require.NoError(t, err)
	}
	got, err := f.rels.SearchPeople(ctx, f.ids["viewer"], "collector", 20)
	require.NoError(t, err)
	require.Equal(t, []string{"collector", "zed_collector"}, handles(got))
}
