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

// CatalogEntry is the global "known thing" record. Empty table in MVP;
// phase-2 populates it via seeds + user submissions + imports.
type CatalogEntry struct {
	ID          string          `json:"id"`
	CategoryID  string          `json:"category_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Attributes  json.RawMessage `json:"attributes"`
	Source      string          `json:"source"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type Catalog struct {
	pool *pgxpool.Pool
}

func NewCatalog(pool *pgxpool.Pool) *Catalog {
	return &Catalog{pool: pool}
}

const catalogSelectCols = `
	ce.id, ce.category_id, ce.name, ce.description, ce.attributes,
	ce.source, ce.status, ce.created_at, ce.updated_at
`

// Search returns approved catalog entries in the given category, narrowed by
// query (trigram match on name). Unknown category returns ErrNotFound so
// callers can respond with 404 cleanly.
func (c *Catalog) Search(ctx context.Context, categorySlug, query string) ([]CatalogEntry, error) {
	var categoryID string
	err := c.pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, categorySlug).Scan(&categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lookup category: %w", err)
	}

	var rows pgx.Rows
	if query == "" {
		rows, err = c.pool.Query(ctx, `
			SELECT `+catalogSelectCols+`
			FROM catalog_entries ce
			WHERE ce.category_id = $1 AND ce.status = 'approved'
			ORDER BY ce.name ASC
			LIMIT 50
		`, categoryID)
	} else {
		rows, err = c.pool.Query(ctx, `
			SELECT `+catalogSelectCols+`
			FROM catalog_entries ce
			WHERE ce.category_id = $1 AND ce.status = 'approved'
			  AND ce.name ILIKE '%' || $2 || '%'
			ORDER BY similarity(ce.name, $2) DESC, ce.name ASC
			LIMIT 50
		`, categoryID, query)
	}
	if err != nil {
		return nil, fmt.Errorf("search catalog: %w", err)
	}
	defer rows.Close()

	out := make([]CatalogEntry, 0)
	for rows.Next() {
		e, err := scanCatalogEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanCatalogEntry(r rowScanner) (CatalogEntry, error) {
	var e CatalogEntry
	var raw []byte
	if err := r.Scan(
		&e.ID, &e.CategoryID, &e.Name, &e.Description, &raw,
		&e.Source, &e.Status, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return CatalogEntry{}, err
	}
	e.Attributes = json.RawMessage(raw)
	return e, nil
}
