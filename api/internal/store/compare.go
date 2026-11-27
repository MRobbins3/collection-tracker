package store

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// ErrCompareSelf: a comparison needs two different people.
var ErrCompareSelf = errors.New("store: can't compare with yourself")

// CompareBucket is one of the four views of a comparison.
type CompareBucket string

const (
	BucketBoth       CompareBucket = "both"
	BucketMineOnly   CompareBucket = "mine_only"
	BucketTheirsOnly CompareBucket = "theirs_only"
	// BucketNeither is the category's approved catalog entries that
	// neither person has.
	BucketNeither CompareBucket = "neither"
)

// ParseCompareBucket accepts exactly the four bucket names.
func ParseCompareBucket(s string) (CompareBucket, bool) {
	switch b := CompareBucket(s); b {
	case BucketBoth, BucketMineOnly, BucketTheirsOnly, BucketNeither:
		return b, true
	}
	return "", false
}

// CompareCategory is one entry in the comparison's category picker.
type CompareCategory struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Public     bool   `json:"public"`
	MineItems  int    `json:"mine_items"`
	TheirItems int    `json:"their_items"`
}

// CompareQuery asks for one page of one bucket. The caller must already have
// applied the privacy rule: the viewer may see OtherID's collections.
type CompareQuery struct {
	ViewerID     string
	OtherID      string
	CategorySlug string
	Bucket       CompareBucket
	Limit        int
	Offset       int
}

// CompareSide is what one person has of a matched thing: every copy across
// all their collections in the category.
type CompareSide struct {
	// Quantity is the sum of the copies' quantities; Items counts the copies.
	Quantity   int      `json:"quantity"`
	Items      int      `json:"items"`
	Conditions []string `json:"conditions"`
	// CollectionID / CollectionName are where the earliest copy lives.
	CollectionID   string `json:"collection_id"`
	CollectionName string `json:"collection_name"`
}

// CompareRow is one matched thing. Key is the match key ("entry:<id>" or
// "name:<normalized name>"), stable and unique within a comparison; it's for
// list keys, never for display.
type CompareRow struct {
	Key            string       `json:"key"`
	Name           string       `json:"name"`
	Identifier     *string      `json:"identifier,omitempty"`
	CatalogEntryID *string      `json:"catalog_entry_id,omitempty"`
	Mine           *CompareSide `json:"mine"`
	Theirs         *CompareSide `json:"theirs"`
	// Entry is the full catalog entry, on "neither" rows only, so the viewer
	// can add it to a collection straight away.
	Entry *CatalogEntry `json:"entry,omitempty"`
}

// CompareCounts is the size of every bucket.
type CompareCounts struct {
	Both       int `json:"both"`
	MineOnly   int `json:"mine_only"`
	TheirsOnly int `json:"theirs_only"`
	Neither    int `json:"neither"`
}

// CompareCategoryInfo describes the compared category.
type CompareCategoryInfo struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
	// CatalogEntries is how many approved catalog entries it has; "neither"
	// needs a catalog.
	CatalogEntries int `json:"catalog_entries"`
}

// CompareResult is the counts plus one page of the requested bucket.
type CompareResult struct {
	Category CompareCategoryInfo `json:"category"`
	Counts   CompareCounts       `json:"counts"`
	Bucket   CompareBucket       `json:"bucket"`
	Items    []CompareRow        `json:"items"`
	HasMore  bool                `json:"has_more"`
}

type Compare struct {
	pool *pgxpool.Pool
}

func NewCompare(pool *pgxpool.Pool) *Compare {
	return &Compare{pool: pool}
}

// compareVisible is the category rule for a comparison: visibleTo for the
// viewer ($1), widened to the other person's own categories ($2), which the
// viewer may see through their collections (ADR 0013). A third person's
// unapproved category stays hidden, with everything in it.
const compareVisible = `(cat.status = 'approved' OR cat.created_by = $1::uuid OR cat.created_by = $2::uuid)`

// Categories lists the categories either person has items in that the
// viewer may see, by name.
func (c *Compare) Categories(ctx context.Context, viewerID, otherID string) ([]CompareCategory, error) {
	if !validIDs(viewerID, otherID) {
		return []CompareCategory{}, nil
	}
	rows, err := c.pool.Query(ctx, `
		SELECT cat.slug, cat.name, cat.status = 'approved',
		       count(i.id) FILTER (WHERE col.user_id = $1),
		       count(i.id) FILTER (WHERE col.user_id = $2)
		FROM categories cat
		JOIN collections col ON col.category_id = cat.id AND col.user_id IN ($1, $2)
		JOIN items i ON i.collection_id = col.id
		WHERE `+compareVisible+`
		GROUP BY cat.id
		ORDER BY lower(cat.name), cat.slug`, viewerID, otherID)
	if err != nil {
		return nil, fmt.Errorf("compare categories: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[CompareCategory])
}

// ownedCopy is one item, from either side, in the compared category.
type ownedCopy struct {
	mine           bool
	entryID        *string
	name           string
	quantity       int
	condition      *string
	collectionID   string
	collectionName string
}

// Compare matches the two people's items in one category and returns the
// counts plus one page of the requested bucket.
//
// Matching: an item linked to a catalog entry matches by that entry. A
// free-form item whose normalized name (textnorm.Key) names exactly one
// approved entry in the category counts as that entry; any other free-form
// item matches by normalized name. Owning one copy is enough; quantities
// and copies are summed per side for display.
//
// Errors: ErrCompareSelf, ErrNotFound (unknown or hidden category).
func (c *Compare) Compare(ctx context.Context, q CompareQuery) (CompareResult, error) {
	if q.ViewerID == q.OtherID {
		return CompareResult{}, ErrCompareSelf
	}
	if !validIDs(q.ViewerID, q.OtherID) {
		return CompareResult{}, ErrNotFound
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	var info CompareCategoryInfo
	var categoryID string
	err := c.pool.QueryRow(ctx, `
		SELECT cat.id, cat.slug, cat.name, cat.status = 'approved',
		       (SELECT count(*) FROM catalog_entries ce WHERE ce.category_id = cat.id AND ce.status = 'approved')
		FROM categories cat
		WHERE cat.slug = $3 AND `+compareVisible, q.ViewerID, q.OtherID, q.CategorySlug,
	).Scan(&categoryID, &info.Slug, &info.Name, &info.Public, &info.CatalogEntries)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CompareResult{}, ErrNotFound
		}
		return CompareResult{}, fmt.Errorf("compare category: %w", err)
	}

	copies, err := c.copies(ctx, categoryID, q.ViewerID, q.OtherID)
	if err != nil {
		return CompareResult{}, err
	}
	byName, err := c.uniqueEntriesByName(ctx, categoryID, copies)
	if err != nil {
		return CompareResult{}, err
	}
	rows, ownedEntries := groupCopies(copies, byName)
	if err := c.labelEntryRows(ctx, rows); err != nil {
		return CompareResult{}, err
	}

	buckets := map[CompareBucket][]*CompareRow{}
	for _, r := range rows {
		switch {
		case r.Mine != nil && r.Theirs != nil:
			buckets[BucketBoth] = append(buckets[BucketBoth], r)
		case r.Mine != nil:
			buckets[BucketMineOnly] = append(buckets[BucketMineOnly], r)
		default:
			buckets[BucketTheirsOnly] = append(buckets[BucketTheirsOnly], r)
		}
	}
	neither, err := c.neitherCount(ctx, categoryID, ownedEntries)
	if err != nil {
		return CompareResult{}, err
	}
	res := CompareResult{
		Category: info,
		Counts: CompareCounts{
			Both: len(buckets[BucketBoth]), MineOnly: len(buckets[BucketMineOnly]),
			TheirsOnly: len(buckets[BucketTheirsOnly]), Neither: neither,
		},
		Bucket: q.Bucket,
		Items:  []CompareRow{},
	}

	if q.Bucket == BucketNeither {
		res.Items, res.HasMore, err = c.neitherPage(ctx, categoryID, ownedEntries, q.Limit, q.Offset)
		return res, err
	}
	list := buckets[q.Bucket]
	sortRows(list)
	for i := q.Offset; i < len(list) && i < q.Offset+q.Limit; i++ {
		res.Items = append(res.Items, *list[i])
	}
	res.HasMore = q.Offset+q.Limit < len(list)
	return res, nil
}

// copies loads both people's items in the category, earliest first.
func (c *Compare) copies(ctx context.Context, categoryID, viewerID, otherID string) ([]ownedCopy, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT col.user_id = $2, i.catalog_entry_id, i.name, i.quantity, i.condition, col.id, col.name
		FROM items i
		JOIN collections col ON col.id = i.collection_id
		WHERE col.category_id = $1 AND col.user_id IN ($2, $3)
		ORDER BY i.created_at, i.id`, categoryID, viewerID, otherID)
	if err != nil {
		return nil, fmt.Errorf("compare items: %w", err)
	}
	defer rows.Close()
	out := []ownedCopy{}
	for rows.Next() {
		var o ownedCopy
		if err := rows.Scan(&o.mine, &o.entryID, &o.name, &o.quantity, &o.condition,
			&o.collectionID, &o.collectionName); err != nil {
			return nil, fmt.Errorf("scan compare item: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// uniqueEntriesByName maps each free-form item's normalized name to the
// approved entry it names, when exactly one entry in the category has that
// name. Ambiguous names (two "Batmobile" sets) map to nothing.
func (c *Compare) uniqueEntriesByName(ctx context.Context, categoryID string, copies []ownedCopy) (map[string]string, error) {
	keys := []string{}
	seen := map[string]bool{}
	for _, o := range copies {
		k := textnorm.Key(o.name)
		if o.entryID == nil && !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	out := map[string]string{}
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := c.pool.Query(ctx, `
		SELECT name_key, min(id::text)
		FROM catalog_entries
		WHERE category_id = $1 AND status = 'approved' AND name_key = ANY($2)
		GROUP BY name_key
		HAVING count(*) = 1`, categoryID, keys)
	if err != nil {
		return nil, fmt.Errorf("compare entry names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k, id string
		if err := rows.Scan(&k, &id); err != nil {
			return nil, fmt.Errorf("scan compare entry name: %w", err)
		}
		out[k] = id
	}
	return out, rows.Err()
}

// groupCopies folds copies into one row per match key, and returns the set
// of catalog entries either person owns.
func groupCopies(copies []ownedCopy, byName map[string]string) ([]*CompareRow, map[string]bool) {
	rows := []*CompareRow{}
	byKey := map[string]*CompareRow{}
	owned := map[string]bool{}
	conds := map[*CompareSide]map[string]bool{}
	for _, o := range copies {
		nameKey := textnorm.Key(o.name)
		entryID := o.entryID
		if entryID == nil {
			if id, ok := byName[nameKey]; ok {
				entryID = &id
			}
		}
		key := "name:" + nameKey
		if entryID != nil {
			key = "entry:" + *entryID
			owned[*entryID] = true
		}
		r, ok := byKey[key]
		if !ok {
			r = &CompareRow{Key: key, Name: textnorm.Clean(o.name), CatalogEntryID: entryID}
			byKey[key] = r
			rows = append(rows, r)
		}
		side := &r.Theirs
		if o.mine {
			side = &r.Mine
			// The viewer's own spelling wins for free-form rows.
			if r.Mine == nil && entryID == nil {
				r.Name = textnorm.Clean(o.name)
			}
		}
		if *side == nil {
			*side = &CompareSide{Conditions: []string{}, CollectionID: o.collectionID, CollectionName: o.collectionName}
			conds[*side] = map[string]bool{}
		}
		s := *side
		s.Quantity += o.quantity
		s.Items++
		if o.condition != nil {
			if cond := textnorm.Clean(*o.condition); cond != "" && !conds[s][cond] {
				conds[s][cond] = true
				s.Conditions = append(s.Conditions, cond)
			}
		}
	}
	for s := range conds {
		sort.Strings(s.Conditions)
	}
	return rows, owned
}

// labelEntryRows names catalog-matched rows after their entry (name and
// identifier), whatever the owners called their copies.
func (c *Compare) labelEntryRows(ctx context.Context, rows []*CompareRow) error {
	ids := []string{}
	for _, r := range rows {
		if r.CatalogEntryID != nil {
			ids = append(ids, *r.CatalogEntryID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	res, err := c.pool.Query(ctx, `SELECT id::text, name, identifier FROM catalog_entries WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return fmt.Errorf("compare entry labels: %w", err)
	}
	defer res.Close()
	type label struct {
		name       string
		identifier *string
	}
	labels := map[string]label{}
	for res.Next() {
		var id string
		var l label
		if err := res.Scan(&id, &l.name, &l.identifier); err != nil {
			return fmt.Errorf("scan compare entry label: %w", err)
		}
		labels[id] = l
	}
	if err := res.Err(); err != nil {
		return err
	}
	for _, r := range rows {
		if r.CatalogEntryID == nil {
			continue
		}
		if l, ok := labels[*r.CatalogEntryID]; ok {
			r.Name = textnorm.Clean(l.name)
			r.Identifier = l.identifier
		}
	}
	return nil
}

// sortRows orders a bucket by normalized name, then identifier, then the
// unique match key, so pages never overlap or skip.
func sortRows(rows []*CompareRow) {
	ident := func(r *CompareRow) string {
		if r.Identifier == nil {
			return ""
		}
		return textnorm.Key(*r.Identifier)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if ka, kb := textnorm.Key(a.Name), textnorm.Key(b.Name); ka != kb {
			return ka < kb
		}
		if ia, ib := ident(a), ident(b); ia != ib {
			return ia < ib
		}
		return a.Key < b.Key
	})
}

func ownedIDs(owned map[string]bool) []string {
	ids := make([]string, 0, len(owned))
	for id := range owned {
		ids = append(ids, id)
	}
	return ids
}

// neitherCount counts the category's approved entries neither person owns.
func (c *Compare) neitherCount(ctx context.Context, categoryID string, owned map[string]bool) (int, error) {
	var n int
	err := c.pool.QueryRow(ctx, `
		SELECT count(*) FROM catalog_entries ce
		WHERE ce.category_id = $1 AND ce.status = 'approved' AND NOT (ce.id = ANY($2::uuid[]))`,
		categoryID, ownedIDs(owned)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("compare neither count: %w", err)
	}
	return n, nil
}

// neitherPage is one page of the category's approved entries that neither
// person owns, in catalog order (name, identifier, id).
func (c *Compare) neitherPage(ctx context.Context, categoryID string, owned map[string]bool, limit, offset int) ([]CompareRow, bool, error) {
	rows, err := c.pool.Query(ctx, `
		SELECT `+catalogSelectCols+`
		FROM catalog_entries ce
		JOIN categories cat ON cat.id = ce.category_id
		WHERE ce.category_id = $1 AND ce.status = 'approved' AND NOT (ce.id = ANY($2::uuid[]))
		ORDER BY ce.name_key, ce.identifier_key, ce.id
		LIMIT $3 OFFSET $4`, categoryID, ownedIDs(owned), limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("compare neither: %w", err)
	}
	defer rows.Close()
	out := []CompareRow{}
	for rows.Next() {
		e, err := scanCatalogEntry(rows)
		if err != nil {
			return nil, false, fmt.Errorf("scan compare neither: %w", err)
		}
		id := e.ID
		out = append(out, CompareRow{
			Key: "entry:" + e.ID, Name: e.Name, Identifier: e.Identifier, CatalogEntryID: &id, Entry: &e,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	return out, hasMore, nil
}
