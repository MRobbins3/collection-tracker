package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/profile"
)

type User struct {
	ID          string `json:"id"`
	GoogleSub   string `json:"-"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Handle      string `json:"handle"`
	AvatarColor string `json:"avatar_color"`
	// Role is nil for almost everyone; "admin" for moderators (ADR 0004).
	// /me exposes it so the web can show admin navigation; the API checks
	// it server-side on every admin request (auth.RequireRole).
	Role      *string   `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

const userCols = `id, google_sub, email, display_name, handle, role, created_at`

// maxHandleAttempts bounds the retry when two sign-ups race for the same
// derived handle.
const maxHandleAttempts = 5

// UpsertByGoogleSub returns the user with this Google `sub`, creating them on
// first sign-in. Email is refreshed on every sign-in so Google changes flow
// through; the display name and handle are the user's to edit (settings
// page), so only a new account takes them from Google. A new account gets a
// handle derived from its display name, with a numeric suffix if taken.
func (u *Users) UpsertByGoogleSub(ctx context.Context, sub, email, displayName string) (User, error) {
	row := u.pool.QueryRow(ctx,
		`UPDATE users SET email = $2 WHERE google_sub = $1 RETURNING `+userCols, sub, email)
	user, err := scanUser(row)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, fmt.Errorf("refresh user: %w", err)
	}

	name := profile.NormalizeDisplayName(displayName)
	if r := []rune(name); len(r) > profile.DisplayNameMaxLen {
		name = strings.TrimSpace(string(r[:profile.DisplayNameMaxLen]))
	}
	if name == "" {
		name = "Collector"
	}
	base := profile.DeriveHandleBase(name)
	for attempt := 0; attempt < maxHandleAttempts; attempt++ {
		handle, err := u.freeHandle(ctx, base)
		if err != nil {
			return User{}, err
		}
		row := u.pool.QueryRow(ctx, `
			INSERT INTO users (google_sub, email, display_name, handle)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (google_sub) DO UPDATE SET email = EXCLUDED.email
			RETURNING `+userCols, sub, email, name, handle)
		user, err = scanUser(row)
		if err == nil {
			return user, nil
		}
		if !isUniqueViolation(err, "users_handle_key") {
			return User{}, fmt.Errorf("insert user: %w", err)
		}
		// Someone took the handle between our check and insert; pick again.
	}
	return User{}, fmt.Errorf("insert user: no free handle for %q after %d attempts", base, maxHandleAttempts)
}

// freeHandle returns base, or base plus the lowest numeric suffix that's
// neither taken nor reserved.
func (u *Users) freeHandle(ctx context.Context, base string) (string, error) {
	rows, err := u.pool.Query(ctx,
		`SELECT handle FROM users WHERE handle = $1 OR handle ~ ('^' || $1 || '[0-9]+$')`, base)
	if err != nil {
		return "", fmt.Errorf("find free handle: %w", err)
	}
	existing, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", fmt.Errorf("find free handle: %w", err)
	}
	taken := make(map[string]bool, len(existing))
	for _, h := range existing {
		taken[h] = true
	}
	return profile.PickFreeHandle(base, taken), nil
}

func (u *Users) GetByID(ctx context.Context, id string) (User, error) {
	row := u.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user %q: %w", id, err)
	}
	return user, nil
}

// GetByGoogleSub returns the user with the given google_sub, or ErrNotFound.
func (u *Users) GetByGoogleSub(ctx context.Context, sub string) (User, error) {
	row := u.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE google_sub = $1`, sub)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by sub: %w", err)
	}
	return user, nil
}

// Delete removes the user and, through ON DELETE CASCADE, everything they
// own (collections, items, moderation requests). Their unpublished catalog
// submissions (pending or rejected) are deleted too; approved catalog
// entries they submitted or approved stay, unattributed (ON DELETE SET
// NULL). Tables added later that reference users must cascade too (ADR
// 0009). ErrNotFound if there's no such user.
func (u *Users) Delete(ctx context.Context, id string) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Lock the user first: Submit holds the same lock while it inserts, so a
	// suggestion can't land between deleting unpublished entries and the
	// user (which would leave a pending entry with no submitter).
	var one int
	err = tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM catalog_entries WHERE submitted_by = $1 AND status <> 'approved'`, id); err != nil {
		return fmt.Errorf("delete unpublished submissions: %w", err)
	}
	// The user's unapproved categories go with them (ADR 0013); only they
	// could use them, so their collections are the user's own. Approved
	// categories stay, unattributed (created_by is SET NULL).
	if _, err := tx.Exec(ctx, `
		DELETE FROM collections c USING categories cat
		WHERE c.category_id = cat.id AND cat.created_by = $1 AND cat.status <> 'approved'`, id); err != nil {
		return fmt.Errorf("delete collections in unpublished categories: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM categories WHERE created_by = $1 AND status <> 'approved'`, id); err != nil {
		return fmt.Errorf("delete unpublished categories: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// DevSubPrefix marks persona users created by the dev seed set
// (sql/dev/0001_dev_users.sql). Real Google subject ids are numeric, so a
// 'dev:' sub can never belong to a real account. The rest of the sub is the
// persona's handle.
const DevSubPrefix = "dev:"

// Persona is a dev-seeded user offered by the dev sign-in picker, with
// counts so the picker can say what each persona has.
type Persona struct {
	Handle          string `json:"handle"`
	DisplayName     string `json:"display_name"`
	Email           string `json:"email"`
	CollectionCount int    `json:"collection_count"`
	ItemCount       int    `json:"item_count"`
}

// ListDevPersonas returns every 'dev:' user, ordered by display name.
func (u *Users) ListDevPersonas(ctx context.Context) ([]Persona, error) {
	rows, err := u.pool.Query(ctx, `
		SELECT substr(u.google_sub, length($1) + 1), u.display_name, u.email,
		       count(DISTINCT c.id)::int, count(i.id)::int
		FROM users u
		LEFT JOIN collections c ON c.user_id = u.id
		LEFT JOIN items i ON i.collection_id = c.id
		WHERE starts_with(u.google_sub, $1)
		GROUP BY u.id
		ORDER BY u.display_name, u.google_sub
	`, DevSubPrefix)
	if err != nil {
		return nil, fmt.Errorf("list dev personas: %w", err)
	}
	personas, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Persona])
	if err != nil {
		return nil, fmt.Errorf("list dev personas: %w", err)
	}
	return personas, nil
}

func scanUser(r rowScanner) (User, error) {
	var u User
	if err := r.Scan(&u.ID, &u.GoogleSub, &u.Email, &u.DisplayName, &u.Handle, &u.Role, &u.CreatedAt); err != nil {
		return User{}, err
	}
	u.AvatarColor = profile.AvatarColor(u.ID)
	return u, nil
}
