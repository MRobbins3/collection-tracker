package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/privacy"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/profile"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// Relationships is friends and followers (ADR 0014), and answers "how does
// the viewer relate to the owner?" for the privacy rule (package privacy,
// ADR 0009).
//
// Every write is a single statement keyed on the pair, so it is idempotent
// and safe under concurrency: friendships store each unordered pair once
// (user_a < user_b), and follows one row per direction.
type Relationships struct {
	pool *pgxpool.Pool
}

func NewRelationships(pool *pgxpool.Pool) *Relationships {
	return &Relationships{pool: pool}
}

var (
	// ErrNotAccepting: the other person has turned that kind of request off.
	ErrNotAccepting = errors.New("store: not accepting requests")
	// ErrSelfRelationship: you can't befriend or follow yourself.
	ErrSelfRelationship = errors.New("store: relationship with yourself")
)

// FriendStatus is the friendship between two people, from one side.
type FriendStatus string

const (
	FriendNone            FriendStatus = "none"
	FriendRequestSent     FriendStatus = "request_sent"
	FriendRequestReceived FriendStatus = "request_received"
	Friends               FriendStatus = "friends"
)

// FollowStatus is one direction of following.
type FollowStatus string

const (
	FollowNone      FollowStatus = "none"
	FollowRequested FollowStatus = "requested"
	FollowFollowing FollowStatus = "following"
)

// RelationshipState is everything between the viewer and another person,
// from the viewer's side, so the UI can show the right buttons.
type RelationshipState struct {
	Friend     FriendStatus `json:"friend"`
	Following  FollowStatus `json:"following"`   // viewer → them
	FollowsYou FollowStatus `json:"follows_you"` // them → viewer
}

// Person is someone in a friends, followers, requests, or search list: only
// the public card fields. Since is when the relationship (or request) began.
type Person struct {
	UserID      string    `json:"-"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	AvatarColor string    `json:"avatar_color"`
	Since       time.Time `json:"since"`
}

// Requests is everything pending that involves one user.
type Requests struct {
	FriendIncoming []Person `json:"friend_incoming"`
	FriendOutgoing []Person `json:"friend_outgoing"`
	FollowIncoming []Person `json:"follow_incoming"`
	FollowOutgoing []Person `json:"follow_outgoing"`
}

// pairArgs orders two ids the way friendships stores them.
func pairArgs(x, y string) (string, string) {
	if y < x {
		return y, x
	}
	return x, y
}

// relationshipSQL is the privacy relationship of viewer $1 to owner u.id,
// closest tie first. A request the owner sent the viewer is Pending; one
// the viewer sent is nothing. Between and SearchPeople share it so search
// can never disagree with the rule.
const relationshipSQL = `CASE
	WHEN EXISTS (SELECT 1 FROM friendships f
	             WHERE f.user_a = least($1::uuid, u.id) AND f.user_b = greatest($1::uuid, u.id)
	               AND f.status = 'accepted') THEN 'friend'
	WHEN EXISTS (SELECT 1 FROM follows fo
	             WHERE fo.follower_id = $1 AND fo.followee_id = u.id AND fo.status = 'approved') THEN 'follower'
	WHEN EXISTS (SELECT 1 FROM friendships f
	             WHERE f.user_a = least($1::uuid, u.id) AND f.user_b = greatest($1::uuid, u.id)
	               AND f.status = 'pending' AND f.requested_by = u.id)
	  OR EXISTS (SELECT 1 FROM follows fo
	             WHERE fo.follower_id = u.id AND fo.followee_id = $1 AND fo.status = 'pending') THEN 'pending'
	ELSE 'none' END`

// Between returns the viewer's relationship to the owner. viewerID is ""
// for anonymous viewers.
func (r *Relationships) Between(ctx context.Context, viewerID, ownerID string) (privacy.Relationship, error) {
	if viewerID == "" || viewerID == ownerID {
		return privacy.RelationshipFor(viewerID, ownerID, privacy.None), nil
	}
	var rel string
	err := r.pool.QueryRow(ctx, `SELECT `+relationshipSQL+` FROM users u WHERE u.id = $2`,
		viewerID, ownerID).Scan(&rel)
	if errors.Is(err, pgx.ErrNoRows) {
		return privacy.None, nil
	}
	if err != nil {
		return privacy.None, fmt.Errorf("relationship between: %w", err)
	}
	return privacy.RelationshipFor(viewerID, ownerID, privacy.Relationship(rel)), nil
}

// State reports the friendship and both follow directions between the
// viewer and other.
func (r *Relationships) State(ctx context.Context, viewerID, otherID string) (RelationshipState, error) {
	a, b := pairArgs(viewerID, otherID)
	var friendStatus, requestedBy, following, followsYou *string
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT status FROM friendships WHERE user_a = $1 AND user_b = $2),
			(SELECT requested_by::text FROM friendships WHERE user_a = $1 AND user_b = $2),
			(SELECT status FROM follows WHERE follower_id = $3 AND followee_id = $4),
			(SELECT status FROM follows WHERE follower_id = $4 AND followee_id = $3)`,
		a, b, viewerID, otherID,
	).Scan(&friendStatus, &requestedBy, &following, &followsYou)
	if err != nil {
		return RelationshipState{}, fmt.Errorf("relationship state: %w", err)
	}
	return RelationshipState{
		Friend:     friendStatusFor(viewerID, friendStatus, requestedBy),
		Following:  followStatusFor(following),
		FollowsYou: followStatusFor(followsYou),
	}, nil
}

func friendStatusFor(viewerID string, status, requestedBy *string) FriendStatus {
	switch {
	case status == nil:
		return FriendNone
	case *status == "accepted":
		return Friends
	case requestedBy != nil && *requestedBy == viewerID:
		return FriendRequestSent
	default:
		return FriendRequestReceived
	}
}

func followStatusFor(status *string) FollowStatus {
	switch {
	case status == nil:
		return FollowNone
	case *status == "approved":
		return FollowFollowing
	default:
		return FollowRequested
	}
}

// SendFriendRequest asks `to` to be friends and returns the friendship
// from `from`'s side. If `to` already asked `from`, they become friends
// (even if `to` has since turned requests off: they asked first). Asking
// again is a no-op that reports the current status, as is asking someone
// you're already friends with. Errors: ErrSelfRelationship, ErrNotAccepting.
func (r *Relationships) SendFriendRequest(ctx context.Context, from, to string) (FriendStatus, error) {
	if from == to {
		return "", ErrSelfRelationship
	}
	a, b := pairArgs(from, to)

	// They already asked: accept.
	tag, err := r.pool.Exec(ctx, `
		UPDATE friendships SET status = 'accepted', accepted_at = now()
		WHERE user_a = $1 AND user_b = $2 AND status = 'pending' AND requested_by = $3`, a, b, to)
	if err != nil {
		return "", fmt.Errorf("send friend request: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return Friends, nil
	}

	accepts, err := r.accepts(ctx, to, "accept_friend_requests")
	if err != nil {
		return "", err
	}
	if !accepts {
		st, err := r.State(ctx, from, to)
		if err != nil {
			return "", err
		}
		if st.Friend == FriendNone {
			return "", ErrNotAccepting
		}
		return st.Friend, nil
	}

	// Insert the request. If `to` inserted theirs between our UPDATE and
	// here, the primary key collides and ON CONFLICT accepts it, so crossing
	// requests always end as one friendship.
	var status, requestedBy string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO friendships (user_a, user_b, requested_by) VALUES ($1, $2, $3)
		ON CONFLICT (user_a, user_b) DO UPDATE SET status = 'accepted', accepted_at = now()
			WHERE friendships.status = 'pending' AND friendships.requested_by <> $3
		RETURNING status, requested_by::text`, a, b, from,
	).Scan(&status, &requestedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict with no update: our own pending request, or friends.
		st, err := r.State(ctx, from, to)
		if err != nil {
			return "", err
		}
		return st.Friend, nil
	}
	if err != nil {
		return "", fmt.Errorf("send friend request: %w", err)
	}
	return friendStatusFor(from, &status, &requestedBy), nil
}

// accepts reads one of the user's "accept … requests" toggles. A missing
// user accepts nothing.
func (r *Relationships) accepts(ctx context.Context, userID, column string) (bool, error) {
	var ok bool
	// column is one of two constants, never user input.
	err := r.pool.QueryRow(ctx, `SELECT `+column+` FROM users WHERE id = $1`, userID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("read %s: %w", column, err)
	}
	return ok, nil
}

// CancelFriendRequest withdraws a pending request `from` sent. No-op when
// there isn't one.
func (r *Relationships) CancelFriendRequest(ctx context.Context, from, to string) error {
	return r.deletePendingFriendship(ctx, from, to, from)
}

// DeclineFriendRequest turns down a pending request `from` sent to `me`.
// No-op when there isn't one.
func (r *Relationships) DeclineFriendRequest(ctx context.Context, me, from string) error {
	return r.deletePendingFriendship(ctx, me, from, from)
}

func (r *Relationships) deletePendingFriendship(ctx context.Context, x, y, requestedBy string) error {
	a, b := pairArgs(x, y)
	_, err := r.pool.Exec(ctx, `
		DELETE FROM friendships
		WHERE user_a = $1 AND user_b = $2 AND status = 'pending' AND requested_by = $3`, a, b, requestedBy)
	if err != nil {
		return fmt.Errorf("delete friend request: %w", err)
	}
	return nil
}

// AcceptFriendRequest accepts a pending request `from` sent to `me`.
// Already friends is a no-op; ErrNotFound when there's no such request
// (only the recipient can accept).
func (r *Relationships) AcceptFriendRequest(ctx context.Context, me, from string) error {
	a, b := pairArgs(me, from)
	var status string
	err := r.pool.QueryRow(ctx, `
		WITH acc AS (
			UPDATE friendships SET status = 'accepted', accepted_at = now()
			WHERE user_a = $1 AND user_b = $2 AND status = 'pending' AND requested_by = $3
			RETURNING status
		)
		SELECT status FROM acc
		UNION ALL
		SELECT status FROM friendships WHERE user_a = $1 AND user_b = $2 AND status = 'accepted'
		LIMIT 1`, a, b, from,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("accept friend request: %w", err)
	}
	return nil
}

// Unfriend ends a friendship; either friend may. No-op when they aren't
// friends (a pending request is left alone).
func (r *Relationships) Unfriend(ctx context.Context, me, other string) error {
	a, b := pairArgs(me, other)
	_, err := r.pool.Exec(ctx,
		`DELETE FROM friendships WHERE user_a = $1 AND user_b = $2 AND status = 'accepted'`, a, b)
	if err != nil {
		return fmt.Errorf("unfriend: %w", err)
	}
	return nil
}

// RequestFollow asks to follow followee; every follow needs approval.
// Asking again reports the current status. Errors: ErrSelfRelationship,
// ErrNotAccepting.
func (r *Relationships) RequestFollow(ctx context.Context, follower, followee string) (FollowStatus, error) {
	if follower == followee {
		return "", ErrSelfRelationship
	}
	accepts, err := r.accepts(ctx, followee, "accept_follow_requests")
	if err != nil {
		return "", err
	}
	if accepts {
		_, err = r.pool.Exec(ctx, `
			INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2)
			ON CONFLICT (follower_id, followee_id) DO NOTHING`, follower, followee)
		if err != nil {
			return "", fmt.Errorf("request follow: %w", err)
		}
	}
	st, err := r.State(ctx, follower, followee)
	if err != nil {
		return "", err
	}
	if st.Following == FollowNone {
		return "", ErrNotAccepting
	}
	return st.Following, nil
}

// Unfollow stops following, or withdraws a pending follow request. No-op
// when there's neither.
func (r *Relationships) Unfollow(ctx context.Context, follower, followee string) error {
	return r.deleteFollow(ctx, follower, followee)
}

// RemoveFollower removes a follower, or denies their pending request. No-op
// when there's neither.
func (r *Relationships) RemoveFollower(ctx context.Context, me, follower string) error {
	return r.deleteFollow(ctx, follower, me)
}

func (r *Relationships) deleteFollow(ctx context.Context, follower, followee string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`, follower, followee)
	if err != nil {
		return fmt.Errorf("delete follow: %w", err)
	}
	return nil
}

// ApproveFollower approves follower's pending request to follow me. Already
// approved is a no-op; ErrNotFound when there's no such request.
func (r *Relationships) ApproveFollower(ctx context.Context, me, follower string) error {
	var status string
	err := r.pool.QueryRow(ctx, `
		WITH app AS (
			UPDATE follows SET status = 'approved', approved_at = now()
			WHERE follower_id = $1 AND followee_id = $2 AND status = 'pending'
			RETURNING status
		)
		SELECT status FROM app
		UNION ALL
		SELECT status FROM follows WHERE follower_id = $1 AND followee_id = $2 AND status = 'approved'
		LIMIT 1`, follower, me,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("approve follower: %w", err)
	}
	return nil
}

// personCols selects a Person from users u plus a `since` expression.
const personCols = `u.id, u.handle, u.display_name`

func (r *Relationships) listPeople(ctx context.Context, sql string, args ...any) ([]Person, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Person, error) {
		var p Person
		if err := row.Scan(&p.UserID, &p.Handle, &p.DisplayName, &p.Since); err != nil {
			return Person{}, err
		}
		p.AvatarColor = profile.AvatarColor(p.UserID)
		return p, nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListFriends returns me's friends, by display name.
func (r *Relationships) ListFriends(ctx context.Context, me string) ([]Person, error) {
	out, err := r.listPeople(ctx, `
		SELECT `+personCols+`, f.accepted_at
		FROM friendships f
		JOIN users u ON u.id = CASE WHEN f.user_a = $1 THEN f.user_b ELSE f.user_a END
		WHERE (f.user_a = $1 OR f.user_b = $1) AND f.status = 'accepted'
		ORDER BY lower(u.display_name), u.handle`, me)
	if err != nil {
		return nil, fmt.Errorf("list friends: %w", err)
	}
	return out, nil
}

// ListFollowers returns the people who follow me (approved), by display name.
func (r *Relationships) ListFollowers(ctx context.Context, me string) ([]Person, error) {
	out, err := r.listPeople(ctx, `
		SELECT `+personCols+`, fo.approved_at
		FROM follows fo JOIN users u ON u.id = fo.follower_id
		WHERE fo.followee_id = $1 AND fo.status = 'approved'
		ORDER BY lower(u.display_name), u.handle`, me)
	if err != nil {
		return nil, fmt.Errorf("list followers: %w", err)
	}
	return out, nil
}

// ListFollowing returns the people me follows (approved), by display name.
func (r *Relationships) ListFollowing(ctx context.Context, me string) ([]Person, error) {
	out, err := r.listPeople(ctx, `
		SELECT `+personCols+`, fo.approved_at
		FROM follows fo JOIN users u ON u.id = fo.followee_id
		WHERE fo.follower_id = $1 AND fo.status = 'approved'
		ORDER BY lower(u.display_name), u.handle`, me)
	if err != nil {
		return nil, fmt.Errorf("list following: %w", err)
	}
	return out, nil
}

// ListRequests returns every pending request to or from me, newest first.
func (r *Relationships) ListRequests(ctx context.Context, me string) (Requests, error) {
	friend := func(incoming bool) ([]Person, error) {
		cond := `f.requested_by = $1`
		if incoming {
			cond = `f.requested_by <> $1`
		}
		return r.listPeople(ctx, `
			SELECT `+personCols+`, f.created_at
			FROM friendships f
			JOIN users u ON u.id = CASE WHEN f.user_a = $1 THEN f.user_b ELSE f.user_a END
			WHERE (f.user_a = $1 OR f.user_b = $1) AND f.status = 'pending' AND `+cond+`
			ORDER BY f.created_at DESC, u.handle`, me)
	}
	follow := func(incoming bool) ([]Person, error) {
		mine, theirs := "fo.follower_id", "fo.followee_id"
		if incoming {
			mine, theirs = theirs, mine
		}
		return r.listPeople(ctx, `
			SELECT `+personCols+`, fo.created_at
			FROM follows fo JOIN users u ON u.id = `+theirs+`
			WHERE `+mine+` = $1 AND fo.status = 'pending'
			ORDER BY fo.created_at DESC, u.handle`, me)
	}
	var out Requests
	var err error
	if out.FriendIncoming, err = friend(true); err != nil {
		return Requests{}, fmt.Errorf("list requests: %w", err)
	}
	if out.FriendOutgoing, err = friend(false); err != nil {
		return Requests{}, fmt.Errorf("list requests: %w", err)
	}
	if out.FollowIncoming, err = follow(true); err != nil {
		return Requests{}, fmt.Errorf("list requests: %w", err)
	}
	if out.FollowOutgoing, err = follow(false); err != nil {
		return Requests{}, fmt.Errorf("list requests: %w", err)
	}
	return out, nil
}

// IncomingRequestCount is how many friend and follow requests wait for me
// to answer (the header badge).
func (r *Relationships) IncomingRequestCount(ctx context.Context, me string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM friendships
			 WHERE (user_a = $1 OR user_b = $1) AND status = 'pending' AND requested_by <> $1)
		  + (SELECT count(*) FROM follows WHERE followee_id = $1 AND status = 'pending')`, me,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("incoming request count: %w", err)
	}
	return n, nil
}

// SearchPeople finds people by handle prefix or display name, among the
// profiles the viewer may see (privacy.CanViewProfile): never the viewer,
// never a Private profile they have no tie to. Exact and prefix handle
// matches rank first. q is trimmed, space-collapsed, and lowercased; one
// leading "@" is ignored.
func (r *Relationships) SearchPeople(ctx context.Context, viewerID, q string, limit int) ([]Person, error) {
	key := textnorm.Key(q)
	handleQ := escapeLike(strings.TrimPrefix(key, "@"))
	nameQ := escapeLike(key)
	if handleQ == "" {
		return []Person{}, nil
	}
	rows, err := r.pool.Query(ctx, `
		WITH cand AS (
			SELECT u.id, u.handle, u.display_name, u.created_at, u.collection_visibility,
			       `+relationshipSQL+` AS rel
			FROM users u
			WHERE u.id <> $1
			  AND (u.handle LIKE $2 || '%' ESCAPE '\'
			       OR lower(u.display_name) LIKE '%' || $3 || '%' ESCAPE '\')
		)
		SELECT id, handle, display_name, created_at, collection_visibility, rel
		FROM cand
		WHERE collection_visibility <> 'private' OR rel <> 'none'
		ORDER BY (handle = $2) DESC, (handle LIKE $2 || '%' ESCAPE '\') DESC, lower(display_name), handle
		LIMIT $4`, viewerID, handleQ, nameQ, limit)
	if err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	defer rows.Close()
	out := []Person{}
	for rows.Next() {
		var p Person
		var vis, rel string
		if err := rows.Scan(&p.UserID, &p.Handle, &p.DisplayName, &p.Since, &vis, &rel); err != nil {
			return nil, fmt.Errorf("search people: %w", err)
		}
		// The SQL narrows; the privacy rule decides.
		if !privacy.CanViewProfile(privacy.Visibility(vis), privacy.Relationship(rel)) {
			continue
		}
		p.AvatarColor = profile.AvatarColor(p.UserID)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	return out, nil
}
