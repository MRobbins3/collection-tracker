// Package privacy is the ONE place that decides who may see a user's profile
// and collections (ADR 0009). Handlers ask it; they never compare visibility
// values themselves.
//
// The rule is a pure function of two inputs: the owner's chosen Visibility
// and the viewer's Relationship to the owner. Looking up the relationship
// is the store's job (store.Relationships.Between, backed by the friendships
// and follows tables; ADR 0014).
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

// Relationship is how the viewer relates to the owner. When several apply,
// the lookup reports the closest tie: Friend, then Follower, then Pending.
//
// A request the VIEWER sent is not a relationship (it stays None until the
// owner says yes), so asking never unlocks anything by itself.
type Relationship string

const (
	None     Relationship = "none"     // anonymous, or signed in with no tie
	Pending  Relationship = "pending"  // owner sent the viewer a friend or follow request
	Follower Relationship = "follower" // viewer follows owner (request approved)
	Friend   Relationship = "friend"   // mutual friendship (request accepted)
	Self     Relationship = "self"     // viewer is the owner
)

// RelationshipFor folds the identity checks every caller needs around a
// stored lookup: the owner viewing themselves is Self and an anonymous
// viewer (empty id) is None, whatever the lookup returned.
func RelationshipFor(viewerID, ownerID string, looked Relationship) Relationship {
	if viewerID == "" {
		return None
	}
	if viewerID == ownerID {
		return Self
	}
	return looked
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
// people who already follow or befriended the owner, nor from someone the
// owner has sent a request to (they need to see who's asking). Pending
// never unlocks collections beyond what a stranger sees.
func CanViewProfile(v Visibility, rel Relationship) bool {
	if rel == Self {
		return true
	}
	if _, known := ParseVisibility(string(v)); !known {
		return false // fail closed
	}
	switch rel {
	case Friend, Follower, Pending:
		return true
	case None:
		return v != Private
	}
	return false
}
