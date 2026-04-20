// Package seed loads initial reference data (currently: the curated category
// list) into the database. It is designed to be idempotent so it can run on
// every API startup without producing duplicates or wiping hand-edits.
package seed

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed data/categories.json
var categoriesJSON []byte

type seedCategory struct {
	Slug            string          `json:"slug"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	AttributeSchema json.RawMessage `json:"attribute_schema"`
}

// Categories upserts the embedded category list. Existing rows keyed by slug
// have their name/description/attribute_schema refreshed; new rows are
// inserted. Returns the number of rows affected.
func Categories(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	var cats []seedCategory
	if err := json.Unmarshal(categoriesJSON, &cats); err != nil {
		return 0, fmt.Errorf("parse embedded categories.json: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var total int64
	for _, c := range cats {
		tag, err := tx.Exec(ctx, `
			INSERT INTO categories (slug, name, description, attribute_schema)
			VALUES ($1, $2, $3, $4::jsonb)
			ON CONFLICT (slug) DO UPDATE SET
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				attribute_schema = EXCLUDED.attribute_schema
		`, c.Slug, c.Name, c.Description, string(c.AttributeSchema))
		if err != nil {
			return 0, fmt.Errorf("upsert category %q: %w", c.Slug, err)
		}
		total += tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit seed tx: %w", err)
	}
	return total, nil
}
