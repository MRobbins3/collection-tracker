package privacy_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
)

var allRelationships = []privacy.Relationship{
	privacy.None, privacy.Pending, privacy.Follower, privacy.Friend, privacy.Self,
}

var allVisibilities = []privacy.Visibility{
	privacy.Public, privacy.FollowersAndFriends, privacy.FriendsOnly, privacy.Private,
}

// TestCanViewCollections spells out the full visibility x relationship
// matrix, so changing the rule means changing this table on purpose.
func TestCanViewCollections(t *testing.T) {
	want := map[privacy.Visibility]map[privacy.Relationship]bool{
		privacy.Public: {
			privacy.None: true, privacy.Pending: true, privacy.Follower: true, privacy.Friend: true, privacy.Self: true,
		},
		privacy.FollowersAndFriends: {
			privacy.None: false, privacy.Pending: false, privacy.Follower: true, privacy.Friend: true, privacy.Self: true,
		},
		privacy.FriendsOnly: {
			privacy.None: false, privacy.Pending: false, privacy.Follower: false, privacy.Friend: true, privacy.Self: true,
		},
		privacy.Private: {
			privacy.None: false, privacy.Pending: false, privacy.Follower: false, privacy.Friend: false, privacy.Self: true,
		},
	}
	for _, v := range allVisibilities {
		for _, rel := range allRelationships {
			t.Run(fmt.Sprintf("%s/%s", v, rel), func(t *testing.T) {
				require.Equal(t, want[v][rel], privacy.CanViewCollections(v, rel))
			})
		}
	}
}

// TestCanViewProfile: the profile card (name, handle, bio) is findable unless
// the owner chose Private; a Private owner is hidden from strangers (404) but
// not from themselves, from people they already have a relationship with, or
// from someone they've sent a request to.
func TestCanViewProfile(t *testing.T) {
	want := map[privacy.Visibility]map[privacy.Relationship]bool{
		privacy.Public: {
			privacy.None: true, privacy.Pending: true, privacy.Follower: true, privacy.Friend: true, privacy.Self: true,
		},
		privacy.FollowersAndFriends: {
			privacy.None: true, privacy.Pending: true, privacy.Follower: true, privacy.Friend: true, privacy.Self: true,
		},
		privacy.FriendsOnly: {
			privacy.None: true, privacy.Pending: true, privacy.Follower: true, privacy.Friend: true, privacy.Self: true,
		},
		privacy.Private: {
			// Pending: a Private owner who sent the viewer a request shows
			// the viewer their card, so the viewer can decide; never more.
			privacy.None: false, privacy.Pending: true, privacy.Follower: true, privacy.Friend: true, privacy.Self: true,
		},
	}
	for _, v := range allVisibilities {
		for _, rel := range allRelationships {
			t.Run(fmt.Sprintf("%s/%s", v, rel), func(t *testing.T) {
				require.Equal(t, want[v][rel], privacy.CanViewProfile(v, rel))
			})
		}
	}
}

// Unknown values must fail closed: only the owner sees anything.
func TestUnknownValuesFailClosed(t *testing.T) {
	bogusVis := privacy.Visibility("everyone-please")
	bogusRel := privacy.Relationship("stalker")
	for _, rel := range []privacy.Relationship{privacy.None, privacy.Pending, privacy.Follower, privacy.Friend} {
		require.False(t, privacy.CanViewCollections(bogusVis, rel), rel)
		require.False(t, privacy.CanViewProfile(bogusVis, rel), rel)
	}
	require.True(t, privacy.CanViewCollections(bogusVis, privacy.Self))
	require.True(t, privacy.CanViewProfile(bogusVis, privacy.Self))
	for _, v := range allVisibilities {
		if v == privacy.Public {
			require.True(t, privacy.CanViewCollections(v, bogusRel))
			continue
		}
		require.False(t, privacy.CanViewCollections(v, bogusRel), v)
	}
	require.False(t, privacy.CanViewProfile(privacy.Private, bogusRel))
}

func TestParseVisibility(t *testing.T) {
	for _, v := range allVisibilities {
		got, ok := privacy.ParseVisibility(string(v))
		require.True(t, ok, v)
		require.Equal(t, v, got)
	}
	for _, bad := range []string{"", "Public", "friends", "everyone"} {
		_, ok := privacy.ParseVisibility(bad)
		require.False(t, ok, bad)
	}
}

func TestDefaultsArePrivacyRespecting(t *testing.T) {
	require.Equal(t, privacy.FriendsOnly, privacy.DefaultVisibility)
	require.False(t, privacy.CanViewCollections(privacy.DefaultVisibility, privacy.None),
		"strangers must not see a new user's collections")
}

func TestRelationshipFor(t *testing.T) {
	require.Equal(t, privacy.Self, privacy.RelationshipFor("u1", "u1", privacy.None))
	require.Equal(t, privacy.None, privacy.RelationshipFor("", "u1", privacy.Friend),
		"an anonymous viewer has no relationship, whatever the lookup says")
	require.Equal(t, privacy.Friend, privacy.RelationshipFor("u2", "u1", privacy.Friend))
	require.Equal(t, privacy.None, privacy.RelationshipFor("u2", "u1", privacy.None))
}
