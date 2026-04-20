package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Category struct {
	ID              string          `json:"id"`
	Slug            string          `json:"slug"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	AttributeSchema json.RawMessage `json:"attribute_schema"`
}

type Categories struct {
	pool *pgxpool.Pool
}

func NewCategories(pool *pgxpool.Pool) *Categories {
	return &Categories{pool: pool}
}

const categorySelectColumns = "id, slug, name, description, attribute_schema"

func (c *Categories) List(ctx context.Context) ([]Category, error) {
	rows, err := c.pool.Query(ctx,
		"SELECT "+categorySelectColumns+" FROM categories ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return collectCategories(rows)
}

// Search uses the trigram index on categories.name to match a fuzzy query.
// Empty query falls back to List.
func (c *Categories) Search(ctx context.Context, q string) ([]Category, error) {
	if q == "" {
		return c.List(ctx)
	}
	rows, err := c.pool.Query(ctx,
		"SELECT "+categorySelectColumns+
			" FROM categories WHERE name ILIKE '%' || $1 || '%' OR slug ILIKE '%' || $1 || '%'"+
			" ORDER BY similarity(name, $1) DESC, name ASC",
		q,
	)
	if err != nil {
		return nil, fmt.Errorf("search categories: %w", err)
	}
	return collectCategories(rows)
}

func (c *Categories) GetBySlug(ctx context.Context, slug string) (Category, error) {
	row := c.pool.QueryRow(ctx,
		"SELECT "+categorySelectColumns+" FROM categories WHERE slug = $1", slug)
	cat, err := scanCategory(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, fmt.Errorf("get category %q: %w", slug, err)
	}
	return cat, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCategory(r rowScanner) (Category, error) {
	var cat Category
	var raw []byte
	if err := r.Scan(&cat.ID, &cat.Slug, &cat.Name, &cat.Description, &raw); err != nil {
		return Category{}, err
	}
	cat.AttributeSchema = json.RawMessage(raw)
	return cat, nil
}

func collectCategories(rows pgx.Rows) ([]Category, error) {
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
