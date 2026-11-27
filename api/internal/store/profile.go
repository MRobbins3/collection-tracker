package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MRobbins3/collection-tracker/api/internal/privacy"
	"github.com/MRobbins3/collection-tracker/api/internal/profile"
)

// Profile is a user's public-facing fields plus their privacy settings.
// Handlers decide which of these a given viewer gets to see.
type Profile struct {
	UserID               string             `json:"-"`
	Handle               string             `json:"handle"`
	DisplayName          string             `json:"display_name"`
	Bio                  string             `json:"bio"`
	AvatarColor          string             `json:"avatar_color"`
	CollectionVisibility privacy.Visibility `json:"collection_visibility"`
	AcceptFriendRequests bool               `json:"accept_friend_requests"`
	AcceptFollowRequests bool               `json:"accept_follow_requests"`
}

// ProfilePatch is a partial update; nil fields are left alone. Handle,
// display name, and bio are normalized here as well as in the handler
// (never trust the caller), and an invalid handle is refused.
type ProfilePatch struct {
	Handle               *string
	DisplayName          *string
	Bio                  *string
	CollectionVisibility *privacy.Visibility
	AcceptFriendRequests *bool
	AcceptFollowRequests *bool
}

var (
	// ErrHandleTaken: another user already has that handle (case-insensitively).
	ErrHandleTaken = errors.New("store: handle taken")
	// ErrInvalidHandle: the handle breaks the format rules or is reserved.
	ErrInvalidHandle = errors.New("store: invalid handle")
	// ErrInvalidProfile: another field breaks its rules.
	ErrInvalidProfile = errors.New("store: invalid profile")
)

const profileCols = `id, handle, display_name, bio, collection_visibility,
	accept_friend_requests, accept_follow_requests`

// GetProfile returns the user's profile and settings, or ErrNotFound.
func (u *Users) GetProfile(ctx context.Context, userID string) (Profile, error) {
	return u.queryProfile(ctx, `SELECT `+profileCols+` FROM users WHERE id = $1`, userID)
}

// GetProfileByHandle looks a profile up by handle, case-insensitively.
// It does NOT apply privacy rules; callers must (see package privacy).
func (u *Users) GetProfileByHandle(ctx context.Context, handle string) (Profile, error) {
	return u.queryProfile(ctx, `SELECT `+profileCols+` FROM users WHERE handle = $1`,
		profile.NormalizeHandle(handle))
}

func (u *Users) queryProfile(ctx context.Context, sql string, arg string) (Profile, error) {
	p, err := scanProfile(u.pool.QueryRow(ctx, sql, arg))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrNotFound
		}
		return Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return p, nil
}

// HandleAvailable reports whether userID could take handle: it's valid, not
// reserved, and nobody else has it (your own current handle is available).
func (u *Users) HandleAvailable(ctx context.Context, handle, userID string) (bool, error) {
	h := profile.NormalizeHandle(handle)
	if profile.ValidateHandle(h) != "" {
		return false, nil
	}
	var takenByOther bool
	err := u.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE handle = $1 AND id <> $2)`, h, userID,
	).Scan(&takenByOther)
	if err != nil {
		return false, fmt.Errorf("handle available: %w", err)
	}
	return !takenByOther, nil
}

// UpdateProfile applies the patch and returns the updated profile.
// Errors: ErrNotFound, ErrInvalidHandle, ErrHandleTaken, ErrInvalidProfile.
func (u *Users) UpdateProfile(ctx context.Context, userID string, p ProfilePatch) (Profile, error) {
	sets := []string{}
	args := []any{userID}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if p.Handle != nil {
		h := profile.NormalizeHandle(*p.Handle)
		if profile.ValidateHandle(h) != "" {
			return Profile{}, ErrInvalidHandle
		}
		add("handle", h)
	}
	if p.DisplayName != nil {
		n := profile.NormalizeDisplayName(*p.DisplayName)
		if profile.ValidateDisplayName(n) != "" {
			return Profile{}, ErrInvalidProfile
		}
		add("display_name", n)
	}
	if p.Bio != nil {
		b := profile.NormalizeBio(*p.Bio)
		if profile.ValidateBio(b) != "" {
			return Profile{}, ErrInvalidProfile
		}
		add("bio", b)
	}
	if p.CollectionVisibility != nil {
		if _, ok := privacy.ParseVisibility(string(*p.CollectionVisibility)); !ok {
			return Profile{}, ErrInvalidProfile
		}
		add("collection_visibility", string(*p.CollectionVisibility))
	}
	if p.AcceptFriendRequests != nil {
		add("accept_friend_requests", *p.AcceptFriendRequests)
	}
	if p.AcceptFollowRequests != nil {
		add("accept_follow_requests", *p.AcceptFollowRequests)
	}
	if len(sets) == 0 {
		return u.GetProfile(ctx, userID)
	}

	// Column names above are constants, never user input.
	sql := `UPDATE users SET ` + strings.Join(sets, ", ") + ` WHERE id = $1 RETURNING ` + profileCols
	out, err := scanProfile(u.pool.QueryRow(ctx, sql, args...))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Profile{}, ErrNotFound
		case isUniqueViolation(err, "users_handle_key"):
			return Profile{}, ErrHandleTaken
		}
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return out, nil
}

func scanProfile(r rowScanner) (Profile, error) {
	var p Profile
	var vis string
	if err := r.Scan(&p.UserID, &p.Handle, &p.DisplayName, &p.Bio, &vis,
		&p.AcceptFriendRequests, &p.AcceptFollowRequests); err != nil {
		return Profile{}, err
	}
	p.CollectionVisibility = privacy.Visibility(vis)
	p.AvatarColor = profile.AvatarColor(p.UserID)
	return p, nil
}

// isUniqueViolation reports whether err is a Postgres unique violation on
// the named constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
