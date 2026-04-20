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

func scanUser(r rowScanner) (User, error) {
	var u User
	if err := r.Scan(&u.ID, &u.GoogleSub, &u.Email, &u.DisplayName, &u.CreatedAt); err != nil {
		return User{}, err
	}
	return u, nil
}
