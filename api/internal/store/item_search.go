package store

import (
	"context"
	"fmt"
	"strings"
)

// ItemSearchResult is one hit from SearchForUser: the item plus the labels
// the UI needs to show where it lives, without a second round-trip.
type ItemSearchResult struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Quantity       int              `json:"quantity"`
	Condition      *string          `json:"condition,omitempty"`
	CollectionID   string           `json:"collection_id"`
	CollectionName string           `json:"collection_name"`
	CategorySlug   string           `json:"category_slug"`
	CategoryName   string           `json:"category_name"`
	MatchedField   *ItemSearchMatch `json:"matched_field,omitempty"`
}

// ItemSearchMatch explains a hit that came from an attribute value rather
// than the item's name. Label is the attribute's human title from the
// category schema, never the raw key.
type ItemSearchMatch struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// SearchForUser finds items in the user's own collections whose name
// contains the query (case-insensitive), whose name is a close fuzzy match
// (pg_trgm word similarity, so small typos still hit), or whose attribute
// values contain the query. Results rank name-prefix matches first, then
// other name matches, then fuzzy name matches, then attribute-only matches.
//
// At most limit results are returned; the bool reports whether more
// matched. An empty query returns no results. Callers normalize whitespace.
//
// Performance: the query walks the caller's items (collections_user_idx,
// then items_collection_idx) and filters each row; the OR with the
// attribute match means the items.name trigram index isn't used. That's
// fine at personal-collection scale.
func (s *Items) SearchForUser(ctx context.Context, userID, q string, limit int) ([]ItemSearchResult, bool, error) {
	out := make([]ItemSearchResult, 0)
	if q == "" || limit <= 0 {
		return out, false, nil
	}
	escaped := escapeLike(q)
	rows, err := s.pool.Query(ctx, `
		SELECT i.id, i.name, i.quantity, i.condition,
		       c.id, c.name, cat.slug, cat.name,
		       am.label, am.value
		FROM items i
		JOIN collections c  ON c.id = i.collection_id
		JOIN categories cat ON cat.id = c.category_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(
			           cat.attribute_schema -> 'properties' -> a.key ->> 'title',
			           initcap(replace(a.key, '_', ' '))
			       ) AS label,
			       a.value
			-- Non-object attributes (never written by the API) would make
			-- jsonb_each_text raise; treat them as no attributes.
			FROM jsonb_each_text(CASE WHEN jsonb_typeof(i.attributes) = 'object'
			                          THEN i.attributes ELSE '{}'::jsonb END) a
			-- Only report an attribute for rows that didn't match on name.
			WHERE i.name NOT ILIKE '%' || $3 || '%' ESCAPE '\'
			  AND NOT ($2 <% i.name)
			  AND a.value ILIKE '%' || $3 || '%' ESCAPE '\'
			ORDER BY a.key
			LIMIT 1
		) am ON true
		WHERE c.user_id = $1
		  AND (i.name ILIKE '%' || $3 || '%' ESCAPE '\'
		       OR $2 <% i.name
		       OR am.value IS NOT NULL)
		ORDER BY
			CASE
				WHEN i.name ILIKE $3 || '%' ESCAPE '\' THEN 0
				WHEN i.name ILIKE '%' || $3 || '%' ESCAPE '\' THEN 1
				WHEN $2 <% i.name THEN 2
				ELSE 3
			END,
			similarity(i.name, $2) DESC,
			lower(i.name),
			i.id
		LIMIT $4
	`, userID, q, escaped, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("search items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r ItemSearchResult
		var label, value *string
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Quantity, &r.Condition,
			&r.CollectionID, &r.CollectionName, &r.CategorySlug, &r.CategoryName,
			&label, &value,
		); err != nil {
			return nil, false, fmt.Errorf("scan item search: %w", err)
		}
		if label != nil && value != nil {
			r.MatchedField = &ItemSearchMatch{Label: *label, Value: *value}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) > limit {
		return out[:limit], true, nil
	}
	return out, false, nil
}

// escapeLike makes LIKE/ILIKE wildcards in user input literal, for use with
// ESCAPE '\'.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
