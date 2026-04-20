package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Item is the user-owned record in a collection. It may reference a catalog
// entry (phase 2) but is fully usable as free-form today (CatalogEntryID nil).
type Item struct {
	ID             string          `json:"id"`
	CollectionID   string          `json:"collection_id"`
	CatalogEntryID *string         `json:"catalog_entry_id,omitempty"`
	Name           string          `json:"name"`
	Quantity       int             `json:"quantity"`
	Condition      *string         `json:"condition,omitempty"`
	Attributes     json.RawMessage `json:"attributes"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// ItemInput is the write-side shape (create + update use the same fields).
// Pointers where omission is meaningfully different from empty.
type ItemInput struct {
	Name       string
	Quantity   int
	Condition  *string
	Attributes json.RawMessage // caller should have validated against the category schema
}

type Items struct {
	pool *pgxpool.Pool
}

func NewItems(pool *pgxpool.Pool) *Items {
	return &Items{pool: pool}
}

const itemSelectCols = `
	i.id, i.collection_id, i.catalog_entry_id, i.name, i.quantity, i.condition,
	i.attributes, i.created_at, i.updated_at
`

// Create inserts a new item in the given collection, owned by the user.
// Returns ErrNotFound if the collection doesn't exist for that user, so
// callers can respond with a single NotFound without leaking existence.
func (s *Items) Create(ctx context.Context, collectionID, userID string, in ItemInput) (Item, error) {
	if err := s.assertCollectionOwned(ctx, collectionID, userID); err != nil {
		return Item{}, err
	}
	attrs := in.Attributes
	if len(attrs) == 0 {
		attrs = json.RawMessage("{}")
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO items (collection_id, name, quantity, condition, attributes)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		RETURNING id
	`, collectionID, in.Name, in.Quantity, in.Condition, string(attrs)).Scan(&id)
	if err != nil {
		return Item{}, fmt.Errorf("insert item: %w", err)
	}
	return s.GetInCollectionForUser(ctx, id, collectionID, userID)
}

// ListInCollectionForUser returns the items in the collection, ordered by
// most-recently-updated. Returns ErrNotFound if the user doesn't own it.
func (s *Items) ListInCollectionForUser(ctx context.Context, collectionID, userID string) ([]Item, error) {
	if err := s.assertCollectionOwned(ctx, collectionID, userID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+itemSelectCols+`
		FROM items i
		WHERE i.collection_id = $1
		ORDER BY i.updated_at DESC
	`, collectionID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()
	out := make([]Item, 0)
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Items) GetInCollectionForUser(ctx context.Context, id, collectionID, userID string) (Item, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+itemSelectCols+`
		FROM items i
		JOIN collections c ON c.id = i.collection_id
		WHERE i.id = $1 AND i.collection_id = $2 AND c.user_id = $3
	`, id, collectionID, userID)
	it, err := scanItem(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrNotFound
		}
		return Item{}, fmt.Errorf("get item: %w", err)
	}
	return it, nil
}

// Update replaces mutable fields on an item owned by the user.
func (s *Items) Update(ctx context.Context, id, collectionID, userID string, in ItemInput) (Item, error) {
	attrs := in.Attributes
	if len(attrs) == 0 {
		attrs = json.RawMessage("{}")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE items
		SET name = $1, quantity = $2, condition = $3, attributes = $4::jsonb, updated_at = now()
		FROM collections c
		WHERE items.id = $5 AND items.collection_id = $6 AND c.id = items.collection_id AND c.user_id = $7
	`, in.Name, in.Quantity, in.Condition, string(attrs), id, collectionID, userID)
	if err != nil {
		return Item{}, fmt.Errorf("update item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Item{}, ErrNotFound
	}
	return s.GetInCollectionForUser(ctx, id, collectionID, userID)
}

func (s *Items) Delete(ctx context.Context, id, collectionID, userID string) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM items
		USING collections c
		WHERE items.id = $1 AND items.collection_id = $2 AND c.id = items.collection_id AND c.user_id = $3
	`, id, collectionID, userID)
	if err != nil {
		return fmt.Errorf("delete item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// assertCollectionOwned returns ErrNotFound if the user doesn't own the
// given collection — so cross-user attempts look like "doesn't exist."
func (s *Items) assertCollectionOwned(ctx context.Context, collectionID, userID string) error {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM collections WHERE id = $1 AND user_id = $2)`,
		collectionID, userID,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check collection: %w", err)
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func scanItem(r rowScanner) (Item, error) {
	var it Item
	var raw []byte
	if err := r.Scan(
		&it.ID, &it.CollectionID, &it.CatalogEntryID,
		&it.Name, &it.Quantity, &it.Condition,
		&raw, &it.CreatedAt, &it.UpdatedAt,
	); err != nil {
		return Item{}, err
	}
	it.Attributes = json.RawMessage(raw)
	return it, nil
}
