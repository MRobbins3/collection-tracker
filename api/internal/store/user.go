package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID          string    `json:"id"`
	GoogleSub   string    `json:"-"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

// UpsertByGoogleSub inserts or refreshes a user row keyed by their Google
// `sub` identifier. Email and display name are refreshed on every login so
// Google profile changes flow through. Returns the resulting row.
func (u *Users) UpsertByGoogleSub(ctx context.Context, sub, email, displayName string) (User, error) {
	row := u.pool.QueryRow(ctx, `
		INSERT INTO users (google_sub, email, display_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (google_sub) DO UPDATE SET
			email = EXCLUDED.email,
			display_name = EXCLUDED.display_name
		RETURNING id, google_sub, email, display_name, created_at
	`, sub, email, displayName)
	return scanUser(row)
}

func (u *Users) GetByID(ctx context.Context, id string) (User, error) {
	row := u.pool.QueryRow(ctx,
		`SELECT id, google_sub, email, display_name, created_at FROM users WHERE id = $1`,
		id,
	)
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
	row := u.pool.QueryRow(ctx,
		`SELECT id, google_sub, email, display_name, created_at FROM users WHERE google_sub = $1`,
		sub,
	)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by sub: %w", err)
	}
	return user, nil
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
	if err := r.Scan(&u.ID, &u.GoogleSub, &u.Email, &u.DisplayName, &u.CreatedAt); err != nil {
		return User{}, err
	}
	return u, nil
}
