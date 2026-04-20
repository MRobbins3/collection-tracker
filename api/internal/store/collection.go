package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Collection is the denormalized row the API returns. Category slug/name are
// joined in so the UI doesn't need a second round-trip.
type Collection struct {
	ID           string    `json:"id"`
	UserID       string    `json:"-"`
	CategoryID   string    `json:"category_id"`
	CategorySlug string    `json:"category_slug"`
	CategoryName string    `json:"category_name"`
	Name         string    `json:"name"`
	ItemCount    int       `json:"item_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Collections struct {
	pool *pgxpool.Pool
}

func NewCollections(pool *pgxpool.Pool) *Collections {
	return &Collections{pool: pool}
}

const collectionSelectCols = `
	c.id, c.user_id, c.category_id, cat.slug, cat.name,
	c.name,
	COALESCE((SELECT COUNT(*) FROM items i WHERE i.collection_id = c.id), 0),
	c.created_at, c.updated_at
`

// ErrCategoryNotFound is returned by Create when the given slug is unknown.
var ErrCategoryNotFound = errors.New("store: category not found")

// Create inserts a new collection owned by the given user, under the given
// category slug. Returns ErrCategoryNotFound if the slug doesn't exist.
func (c *Collections) Create(ctx context.Context, userID, categorySlug, name string) (Collection, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return Collection{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var categoryID string
	err = tx.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, categorySlug).Scan(&categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Collection{}, ErrCategoryNotFound
		}
		return Collection{}, fmt.Errorf("lookup category: %w", err)
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO collections (user_id, category_id, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`, userID, categoryID, name).Scan(&id)
	if err != nil {
		return Collection{}, fmt.Errorf("insert collection: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Collection{}, fmt.Errorf("commit: %w", err)
	}

	return c.GetByIDForUser(ctx, id, userID)
}

// ListByUser returns all collections owned by the user, most-recent first.
func (c *Collections) ListByUser(ctx context.Context, userID string) ([]Collection, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT `+collectionSelectCols+`
		FROM collections c
		JOIN categories cat ON cat.id = c.category_id
		WHERE c.user_id = $1
		ORDER BY c.updated_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	return collectCollections(rows)
}

// GetByIDForUser returns the row only if the user owns it. Missing or
// not-owned both return ErrNotFound to avoid leaking existence.
func (c *Collections) GetByIDForUser(ctx context.Context, id, userID string) (Collection, error) {
	row := c.pool.QueryRow(ctx, `
		SELECT `+collectionSelectCols+`
		FROM collections c
		JOIN categories cat ON cat.id = c.category_id
		WHERE c.id = $1 AND c.user_id = $2
	`, id, userID)
	coll, err := scanCollection(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Collection{}, ErrNotFound
		}
		return Collection{}, fmt.Errorf("get collection: %w", err)
	}
	return coll, nil
}

// Rename updates the name if the user owns the row. ErrNotFound otherwise.
func (c *Collections) Rename(ctx context.Context, id, userID, newName string) (Collection, error) {
	tag, err := c.pool.Exec(ctx, `
		UPDATE collections SET name = $1, updated_at = now()
		WHERE id = $2 AND user_id = $3
	`, newName, id, userID)
	if err != nil {
		return Collection{}, fmt.Errorf("rename collection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Collection{}, ErrNotFound
	}
	return c.GetByIDForUser(ctx, id, userID)
}

// Delete removes the collection if the user owns it. ErrNotFound otherwise.
// Items cascade via the FK definition.
func (c *Collections) Delete(ctx context.Context, id, userID string) error {
	tag, err := c.pool.Exec(ctx, `DELETE FROM collections WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCollection(r rowScanner) (Collection, error) {
	var c Collection
	if err := r.Scan(
		&c.ID, &c.UserID, &c.CategoryID, &c.CategorySlug, &c.CategoryName,
		&c.Name, &c.ItemCount, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return Collection{}, err
	}
	return c, nil
}

func collectCollections(rows pgx.Rows) ([]Collection, error) {
	defer rows.Close()
	out := make([]Collection, 0)
	for rows.Next() {
		c, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
