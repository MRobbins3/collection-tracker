package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OverviewSort is how the collection overview orders a user's items.
type OverviewSort string

const (
	SortNewest   OverviewSort = "newest"
	SortOldest   OverviewSort = "oldest"
	SortNameAsc  OverviewSort = "name_asc"
	SortNameDesc OverviewSort = "name_desc"
	SortCategory OverviewSort = "category"
)

// overviewOrderBy maps each sort to a fixed ORDER BY clause. Every clause
// ends with the item id so paging is stable when the other keys tie.
var overviewOrderBy = map[OverviewSort]string{
	SortNewest:   "i.created_at DESC, i.id DESC",
	SortOldest:   "i.created_at ASC, i.id ASC",
	SortNameAsc:  "lower(i.name) ASC, i.id ASC",
	SortNameDesc: "lower(i.name) DESC, i.id DESC",
	SortCategory: "lower(cat.name) ASC, lower(i.name) ASC, i.id ASC",
}

// ValidOverviewSort reports whether s is one of the known sorts.
func ValidOverviewSort(s OverviewSort) bool {
	_, ok := overviewOrderBy[s]
	return ok
}

// OverviewCategory is one category in a user's overview: a category they
// have at least one collection in, or one they pinned. ItemCount is the
// number of item rows (not the sum of quantities), like Collection.ItemCount.
type OverviewCategory struct {
	Slug            string `json:"slug"`
	Name            string `json:"name"`
	ItemCount       int    `json:"item_count"`
	CollectionCount int    `json:"collection_count"`
	Pinned          bool   `json:"pinned"`
}

// OverviewItem is one of the user's items with the labels the overview needs
// to show where it lives.
type OverviewItem struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Quantity       int       `json:"quantity"`
	Condition      *string   `json:"condition,omitempty"`
	CollectionID   string    `json:"collection_id"`
	CollectionName string    `json:"collection_name"`
	CategorySlug   string    `json:"category_slug"`
	CategoryName   string    `json:"category_name"`
	CreatedAt      time.Time `json:"created_at"`
}

// OverviewQuery selects a page of the user's items. An empty CategorySlugs
// means every category. Callers validate Sort and bound Limit/Offset.
type OverviewQuery struct {
	Sort          OverviewSort
	CategorySlugs []string
	Limit         int
	Offset        int
}

// OverviewItemsPage is one page of items. Total counts every item matching
// the filter, across all pages (0 when Offset is past the end).
type OverviewItemsPage struct {
	Items   []OverviewItem
	Total   int
	HasMore bool
}

// OverviewSettings are the user's overview display preferences.
type OverviewSettings struct {
	ShowEmptyCategories bool `json:"show_empty_categories"`
}

type Overview struct {
	pool *pgxpool.Pool
}

func NewOverview(pool *pgxpool.Pool) *Overview {
	return &Overview{pool: pool}
}

// Categories returns the user's overview categories, by name. Only the
// user's own collections and pins count.
func (o *Overview) Categories(ctx context.Context, userID string) ([]OverviewCategory, error) {
	out := make([]OverviewCategory, 0)
	if !validIDs(userID) {
		return out, nil
	}
	rows, err := o.pool.Query(ctx, `
		WITH mine AS (
			SELECT c.category_id,
			       COUNT(DISTINCT c.id) AS collection_count,
			       COUNT(i.id)          AS item_count
			FROM collections c
			LEFT JOIN items i ON i.collection_id = c.id
			WHERE c.user_id = $1
			GROUP BY c.category_id
		)
		SELECT cat.slug, cat.name,
		       COALESCE(m.item_count, 0), COALESCE(m.collection_count, 0),
		       p.category_id IS NOT NULL
		FROM categories cat
		LEFT JOIN mine m ON m.category_id = cat.id
		LEFT JOIN user_pinned_categories p ON p.category_id = cat.id AND p.user_id = $1
		WHERE (m.category_id IS NOT NULL OR p.category_id IS NOT NULL)
		  AND `+visibleTo+`
		ORDER BY lower(cat.name), cat.slug
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("overview categories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c OverviewCategory
		if err := rows.Scan(&c.Slug, &c.Name, &c.ItemCount, &c.CollectionCount, &c.Pinned); err != nil {
			return nil, fmt.Errorf("scan overview category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Items returns one page of the user's items across all their collections.
func (o *Overview) Items(ctx context.Context, userID string, q OverviewQuery) (OverviewItemsPage, error) {
	page := OverviewItemsPage{Items: make([]OverviewItem, 0)}
	if !validIDs(userID) || q.Limit <= 0 {
		return page, nil
	}
	orderBy, ok := overviewOrderBy[q.Sort]
	if !ok {
		return page, fmt.Errorf("overview items: unknown sort %q", q.Sort)
	}
	var slugs []string
	if len(q.CategorySlugs) > 0 {
		slugs = q.CategorySlugs
	}
	// orderBy comes from the fixed map above, never from the request.
	rows, err := o.pool.Query(ctx, `
		SELECT i.id, i.name, i.quantity, i.condition,
		       c.id, c.name, cat.slug, cat.name, i.created_at,
		       COUNT(*) OVER ()
		FROM items i
		JOIN collections c  ON c.id = i.collection_id
		JOIN categories cat ON cat.id = c.category_id
		WHERE c.user_id = $1
		  AND ($2::text[] IS NULL OR cat.slug = ANY ($2))
		ORDER BY `+orderBy+`
		LIMIT $3 OFFSET $4
	`, userID, slugs, q.Limit+1, q.Offset)
	if err != nil {
		return page, fmt.Errorf("overview items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it OverviewItem
		if err := rows.Scan(
			&it.ID, &it.Name, &it.Quantity, &it.Condition,
			&it.CollectionID, &it.CollectionName, &it.CategorySlug, &it.CategoryName, &it.CreatedAt,
			&page.Total,
		); err != nil {
			return page, fmt.Errorf("scan overview item: %w", err)
		}
		page.Items = append(page.Items, it)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > q.Limit {
		page.Items = page.Items[:q.Limit]
		page.HasMore = true
	}
	return page, nil
}

// Pin adds the category to the user's overview. Pinning an already-pinned
// category is a no-op. ErrCategoryNotFound if the slug is unknown or names
// a category the user can't see (another user's unapproved one; ADR 0013).
func (o *Overview) Pin(ctx context.Context, userID, categorySlug string) error {
	if !validIDs(userID) {
		return ErrNotFound
	}
	tag, err := o.pool.Exec(ctx, `
		INSERT INTO user_pinned_categories (user_id, category_id)
		SELECT $1, cat.id FROM categories cat WHERE cat.slug = $2 AND `+visibleTo+`
		ON CONFLICT (user_id, category_id) DO NOTHING
	`, userID, categorySlug)
	if err != nil {
		return fmt.Errorf("pin category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := o.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM categories cat WHERE cat.slug = $2 AND `+visibleTo+`)`,
			userID, categorySlug).Scan(&exists); err != nil {
			return fmt.Errorf("lookup category: %w", err)
		}
		if !exists {
			return ErrCategoryNotFound
		}
	}
	return nil
}

// Unpin removes the category from the user's pins. Unpinning a category that
// isn't pinned (or doesn't exist) is a no-op. A category the user still has
// collections in stays in their overview.
func (o *Overview) Unpin(ctx context.Context, userID, categorySlug string) error {
	if !validIDs(userID) {
		return nil
	}
	_, err := o.pool.Exec(ctx, `
		DELETE FROM user_pinned_categories p
		USING categories cat
		WHERE p.category_id = cat.id AND p.user_id = $1 AND cat.slug = $2
	`, userID, categorySlug)
	if err != nil {
		return fmt.Errorf("unpin category: %w", err)
	}
	return nil
}

// Settings returns the user's overview settings. ErrNotFound if the user
// doesn't exist.
func (o *Overview) Settings(ctx context.Context, userID string) (OverviewSettings, error) {
	if !validIDs(userID) {
		return OverviewSettings{}, ErrNotFound
	}
	var s OverviewSettings
	err := o.pool.QueryRow(ctx,
		`SELECT show_empty_categories FROM users WHERE id = $1`, userID).Scan(&s.ShowEmptyCategories)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OverviewSettings{}, ErrNotFound
		}
		return OverviewSettings{}, fmt.Errorf("overview settings: %w", err)
	}
	return s, nil
}

// UpdateSettings saves the user's overview settings and returns them.
// ErrNotFound if the user doesn't exist.
func (o *Overview) UpdateSettings(ctx context.Context, userID string, s OverviewSettings) (OverviewSettings, error) {
	if !validIDs(userID) {
		return OverviewSettings{}, ErrNotFound
	}
	var out OverviewSettings
	err := o.pool.QueryRow(ctx, `
		UPDATE users SET show_empty_categories = $2 WHERE id = $1
		RETURNING show_empty_categories
	`, userID, s.ShowEmptyCategories).Scan(&out.ShowEmptyCategories)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OverviewSettings{}, ErrNotFound
		}
		return OverviewSettings{}, fmt.Errorf("update overview settings: %w", err)
	}
	return out, nil
}
