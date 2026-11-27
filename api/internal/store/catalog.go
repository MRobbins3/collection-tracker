package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// ErrCatalogEntryUnavailable is returned when an item would link to a
// catalog entry that doesn't exist, isn't approved, or belongs to a
// different category than the item's collection.
var ErrCatalogEntryUnavailable = errors.New("store: catalog entry unavailable")

// CatalogEntry is the global "known thing" record (ADR 0004). Only approved
// entries ever leave this package.
type CatalogEntry struct {
	ID           string `json:"id"`
	CategoryID   string `json:"category_id"`
	CategorySlug string `json:"category_slug"`
	CategoryName string `json:"category_name"`
	Name         string `json:"name"`
	// Identifier is the value of the category's identifier attribute (set
	// number, artist...), when it has one. Display text, not a key.
	Identifier  *string         `json:"identifier,omitempty"`
	Description *string         `json:"description,omitempty"`
	Attributes  json.RawMessage `json:"attributes"`
	Source      string          `json:"source"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// CatalogQuery selects one page of approved entries in a category. Q is
// matched against the name and identifier; an empty Q lists everything.
type CatalogQuery struct {
	CategorySlug string
	Q            string
	Limit        int
	Offset       int
}

// CatalogPage is one page of entries plus the total matching count.
type CatalogPage struct {
	Entries []CatalogEntry `json:"entries"`
	Total   int            `json:"total"`
	HasMore bool           `json:"has_more"`
}

// CatalogMatch says why an entry was suggested for a free-form item.
type CatalogMatch string

const (
	MatchNameAndIdentifier CatalogMatch = "name_and_identifier"
	MatchIdentifier        CatalogMatch = "identifier"
	MatchName              CatalogMatch = "name"
	MatchSimilarName       CatalogMatch = "similar_name"
)

// similarNameThreshold is the pg_trgm similarity above which a name counts
// as "similar". 0.3 is pg_trgm's own default for the % operator.
const similarNameThreshold = 0.3

// CatalogSuggestion is a likely catalog match for one of the user's items.
type CatalogSuggestion struct {
	CatalogEntry
	Match CatalogMatch `json:"match"`
}

// CatalogOwnership is one of the user's items linked to a catalog entry.
type CatalogOwnership struct {
	CatalogEntryID string `json:"catalog_entry_id"`
	ItemID         string `json:"item_id"`
	CollectionID   string `json:"collection_id"`
	CollectionName string `json:"collection_name"`
}

type Catalog struct {
	pool *pgxpool.Pool
}

func NewCatalog(pool *pgxpool.Pool) *Catalog {
	return &Catalog{pool: pool}
}

const catalogSelectCols = `
	ce.id, ce.category_id, cat.slug, cat.name, ce.name, ce.identifier,
	ce.description, ce.attributes, ce.source, ce.status, ce.created_at, ce.updated_at
`

// List returns one page of approved entries in the category, by name (then
// identifier, then id, so ties page stably), or by match quality when Q is
// set. An unknown category returns ErrNotFound.
func (c *Catalog) List(ctx context.Context, q CatalogQuery) (CatalogPage, error) {
	var categoryID string
	err := c.pool.QueryRow(ctx, `SELECT id FROM categories WHERE slug = $1`, q.CategorySlug).Scan(&categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CatalogPage{}, ErrNotFound
		}
		return CatalogPage{}, fmt.Errorf("lookup category: %w", err)
	}

	key := textnorm.Key(q.Q)
	escaped := escapeLike(key)
	// $2 = '' disables the filter. Matching is on the normalized keys so
	// case and spacing never matter.
	const where = `
		WHERE ce.category_id = $1 AND ce.status = 'approved'
		  AND ($2 = '' OR ce.name_key LIKE '%' || $3 || '%' ESCAPE '\'
		              OR ce.identifier_key LIKE '%' || $3 || '%' ESCAPE '\')
	`

	page := CatalogPage{Entries: make([]CatalogEntry, 0)}
	if err := c.pool.QueryRow(ctx,
		`SELECT count(*) FROM catalog_entries ce`+where, categoryID, key, escaped,
	).Scan(&page.Total); err != nil {
		return CatalogPage{}, fmt.Errorf("count catalog: %w", err)
	}

	rows, err := c.pool.Query(ctx, `
		SELECT `+catalogSelectCols+`
		FROM catalog_entries ce
		JOIN categories cat ON cat.id = ce.category_id
		`+where+`
		ORDER BY
			CASE
				WHEN $2 = '' THEN 0
				WHEN ce.name_key = $2 OR ce.identifier_key = $2 THEN 0
				WHEN ce.name_key LIKE $3 || '%' ESCAPE '\' OR ce.identifier_key LIKE $3 || '%' ESCAPE '\' THEN 1
				ELSE 2
			END,
			ce.name_key, ce.identifier_key, ce.id
		LIMIT $4 OFFSET $5
	`, categoryID, key, escaped, q.Limit+1, q.Offset)
	if err != nil {
		return CatalogPage{}, fmt.Errorf("list catalog: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanCatalogEntry(rows)
		if err != nil {
			return CatalogPage{}, err
		}
		page.Entries = append(page.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return CatalogPage{}, err
	}
	if len(page.Entries) > q.Limit {
		page.Entries = page.Entries[:q.Limit]
		page.HasMore = true
	}
	return page, nil
}

// Get returns one approved entry. Pending, rejected, and unknown entries are
// all ErrNotFound, so unapproved submissions never leak.
func (c *Catalog) Get(ctx context.Context, id string) (CatalogEntry, error) {
	if !validIDs(id) {
		return CatalogEntry{}, ErrNotFound
	}
	e, err := scanCatalogEntry(c.pool.QueryRow(ctx, `
		SELECT `+catalogSelectCols+`
		FROM catalog_entries ce
		JOIN categories cat ON cat.id = ce.category_id
		WHERE ce.id = $1 AND ce.status = 'approved'
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CatalogEntry{}, ErrNotFound
		}
		return CatalogEntry{}, fmt.Errorf("get catalog entry: %w", err)
	}
	return e, nil
}

// Suggest returns up to limit approved entries in the item's category that
// likely describe the user's free-form item: same name and identifier, same
// identifier, same name, or a similar name, in that order. ErrNotFound when
// the item isn't the user's. It only suggests; linking is always the
// owner's explicit choice (SetCatalogEntry).
func (c *Catalog) Suggest(ctx context.Context, itemID, collectionID, userID string, limit int) ([]CatalogSuggestion, error) {
	if !validIDs(itemID, collectionID, userID) {
		return nil, ErrNotFound
	}
	var categoryID, name string
	var identifierAttr *string
	var attrs []byte
	err := c.pool.QueryRow(ctx, `
		SELECT c.category_id, i.name, i.attributes, cat.catalog_identifier
		FROM items i
		JOIN collections c ON c.id = i.collection_id
		JOIN categories cat ON cat.id = c.category_id
		WHERE i.id = $1 AND i.collection_id = $2 AND c.user_id = $3
	`, itemID, collectionID, userID).Scan(&categoryID, &name, &attrs, &identifierAttr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("load item for suggestions: %w", err)
	}

	nameKey := textnorm.Key(name)
	identifierKey := ""
	if identifierAttr != nil {
		var values map[string]any
		if json.Unmarshal(attrs, &values) == nil {
			if v, ok := values[*identifierAttr]; ok && v != nil {
				identifierKey = textnorm.Key(fmt.Sprint(v))
			}
		}
	}

	rows, err := c.pool.Query(ctx, `
		SELECT `+catalogSelectCols+`,
			CASE
				WHEN ce.name_key = $2 AND $3 <> '' AND ce.identifier_key = $3 THEN 'name_and_identifier'
				WHEN $3 <> '' AND ce.identifier_key = $3 THEN 'identifier'
				WHEN ce.name_key = $2 THEN 'name'
				ELSE 'similar_name'
			END AS match
		FROM catalog_entries ce
		JOIN categories cat ON cat.id = ce.category_id
		WHERE ce.category_id = $1 AND ce.status = 'approved'
		  AND (ce.name_key = $2
		       OR ($3 <> '' AND ce.identifier_key = $3)
		       OR similarity(ce.name_key, $2) >= $4)
		ORDER BY
			CASE
				WHEN ce.name_key = $2 AND $3 <> '' AND ce.identifier_key = $3 THEN 0
				WHEN $3 <> '' AND ce.identifier_key = $3 THEN 1
				WHEN ce.name_key = $2 THEN 2
				ELSE 3
			END,
			similarity(ce.name_key, $2) DESC, ce.name_key, ce.identifier_key, ce.id
		LIMIT $5
	`, categoryID, nameKey, identifierKey, similarNameThreshold, limit)
	if err != nil {
		return nil, fmt.Errorf("suggest catalog entries: %w", err)
	}
	defer rows.Close()

	out := make([]CatalogSuggestion, 0)
	for rows.Next() {
		var s CatalogSuggestion
		var raw []byte
		if err := rows.Scan(
			&s.ID, &s.CategoryID, &s.CategorySlug, &s.CategoryName, &s.Name, &s.Identifier,
			&s.Description, &raw, &s.Source, &s.Status, &s.CreatedAt, &s.UpdatedAt, &s.Match,
		); err != nil {
			return nil, err
		}
		s.Attributes = json.RawMessage(raw)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Owned lists the user's items in the category that are linked to a catalog
// entry, so the catalog can mark entries "in your collection". Only the
// caller's own items are ever returned.
func (c *Catalog) Owned(ctx context.Context, userID, categorySlug string) ([]CatalogOwnership, error) {
	if !validIDs(userID) {
		return []CatalogOwnership{}, nil
	}
	rows, err := c.pool.Query(ctx, `
		SELECT i.catalog_entry_id, i.id, c.id, c.name
		FROM items i
		JOIN collections c ON c.id = i.collection_id
		JOIN categories cat ON cat.id = c.category_id
		WHERE c.user_id = $1 AND cat.slug = $2 AND i.catalog_entry_id IS NOT NULL
		ORDER BY lower(c.name), c.id, i.created_at, i.id
	`, userID, categorySlug)
	if err != nil {
		return nil, fmt.Errorf("list owned catalog entries: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[CatalogOwnership])
}

func scanCatalogEntry(r rowScanner) (CatalogEntry, error) {
	var e CatalogEntry
	var raw []byte
	if err := r.Scan(
		&e.ID, &e.CategoryID, &e.CategorySlug, &e.CategoryName, &e.Name, &e.Identifier,
		&e.Description, &raw, &e.Source, &e.Status, &e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return CatalogEntry{}, err
	}
	e.Attributes = json.RawMessage(raw)
	return e, nil
}
