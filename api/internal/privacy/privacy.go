// Package privacy is the ONE place that decides who may see a user's profile
// and collections (ADR 0009). Handlers ask it; they never compare visibility
// values themselves.
//
// The rule is a pure function of two inputs: the owner's chosen Visibility
// and the viewer's Relationship to the owner. Looking up the relationship
// is the store's job (store.Relationships.Between); until friends and
// followers exist it only ever answers Self or None, so Public, Private, and
// "it's me" work today and the follower/friend levels start working as soon
// as that lookup learns about them, with no change here.
package privacy

// Visibility is who an owner lets see their collections. Stored as text in
// users.collection_visibility (a CHECK constraint mirrors these values).
type Visibility string

const (
	Public              Visibility = "public"
	FollowersAndFriends Visibility = "followers_and_friends"
	FriendsOnly         Visibility = "friends_only"
	Private             Visibility = "private"
)

// DefaultVisibility applies to new accounts and was backfilled onto existing
// ones. Friends only: nobody sees a new user's collections until they make
// a friend or opt in to more, but the profile card stays findable so friend
// requests can reach them.
const DefaultVisibility = FriendsOnly

// ParseVisibility accepts exactly the stored spellings.
func ParseVisibility(s string) (Visibility, bool) {
	switch v := Visibility(s); v {
	case Public, FollowersAndFriends, FriendsOnly, Private:
		return v, true
	}
	return "", false
}

// Relationship is how the viewer relates to the owner. When someone is both
// a follower and a friend, the lookup reports Friend (the closer tie).
type Relationship string

const (
	None     Relationship = "none"     // anonymous, or signed in with no tie
	Follower Relationship = "follower" // viewer follows owner (request approved)
	Friend   Relationship = "friend"   // mutual friendship (request accepted)
	Self     Relationship = "self"     // viewer is the owner
)

// RelationshipFor folds the identity checks every caller needs around a
// stored lookup: the owner viewing themselves is Self and an anonymous
// viewer (empty id) is None, whatever the lookup returned.
func RelationshipFor(viewerID, ownerID string, looked Relationship) Relationship {
	switch {
	case viewerID == "":
		return None
	case viewerID == ownerID:
		return Self
	default:
		return looked
	}
}

// CanViewCollections reports whether the viewer may see the owner's
// collections and items. Unknown values fail closed.
func CanViewCollections(v Visibility, rel Relationship) bool {
	if rel == Self {
		return true
	}
	switch v {
	case Public:
		return true
	case FollowersAndFriends:
		return rel == Follower || rel == Friend
	case FriendsOnly:
		return rel == Friend
	default: // Private, or anything unrecognized
		return false
	}
}

// CanViewProfile reports whether the viewer may see the owner's profile card
// (display name, handle, bio, avatar) at all. A hidden profile is answered
// with 404, never 403, so strangers can't tell it exists.
//
// Every level except Private keeps the card findable, so people can send a
// friend or follow request. Private hides it from strangers, but not from
// people who already follow or befriended the owner.
func CanViewProfile(v Visibility, rel Relationship) bool {
	if rel == Self {
		return true
	}
	if _, known := ParseVisibility(string(v)); !known {
		return false // fail closed
	}
	switch rel {
	case Friend, Follower:
		return true
	case None:
		return v != Private
	}
	return false
}
