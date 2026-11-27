package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbins3/collection-tracker/api/internal/category"
	"github.com/MRobbins3/collection-tracker/api/internal/textnorm"
)

// Category statuses (ADR 0013). System categories are always approved.
const (
	CategoryPrivate  = "private"
	CategoryPending  = "pending"
	CategoryApproved = "approved"
	CategoryRejected = "rejected"
)

type Category struct {
	ID              string          `json:"id"`
	Slug            string          `json:"slug"`
	Name            string          `json:"name"`
	Description     *string         `json:"description,omitempty"`
	AttributeSchema json.RawMessage `json:"attribute_schema"`
	// CatalogIdentifier is the attribute that, with the name, tells catalog
	// entries apart (e.g. "set_number"); nil when the name alone does.
	CatalogIdentifier *string `json:"catalog_identifier,omitempty"`
	// Status is approved for every category on the public list; a
	// user-built category is private, pending, or rejected until approved.
	Status string `json:"status"`
	// Mine is true when the viewer built this category.
	Mine bool `json:"mine,omitempty"`
	// ReviewReason is the admin's reason on the creator's latest decided
	// request to list it publicly; only ever filled in for the creator.
	ReviewReason *string `json:"review_reason,omitempty"`
}

// CategoryInput is a user-built category as the builder produced it. The
// handler composes Schema and CatalogIdentifier (package category) and
// validates lengths; the store cleans the text again and enforces
// ownership, status, and name uniqueness.
type CategoryInput struct {
	Name              string
	Description       *string
	Schema            json.RawMessage
	CatalogIdentifier *string
}

var (
	// ErrCategoryLocked: the category is waiting for review or approved,
	// so its creator can no longer edit it (ADR 0013).
	ErrCategoryLocked = errors.New("store: category can't be edited now")
	// ErrCategoryNotApproved: the action needs a category on the public
	// list (e.g. catalog suggestions).
	ErrCategoryNotApproved = errors.New("store: category not approved")
)

// CategoryExistsError: the name matches a category the user can already
// see (case- and space-insensitively). Existing is that category, so the
// caller can offer it instead of a duplicate.
type CategoryExistsError struct {
	Existing Category
}

func (e *CategoryExistsError) Error() string { return "store: category name already in use" }

// SchemaInUseError: an edit would remove a field or option that some of the
// creator's items still use. Message says which.
type SchemaInUseError struct {
	Message string
}

func (e *SchemaInUseError) Error() string { return e.Message }

type Categories struct {
	pool *pgxpool.Pool
}

func NewCategories(pool *pgxpool.Pool) *Categories {
	return &Categories{pool: pool}
}

// visibleTo is THE category visibility rule (ADR 0013): a category is
// visible to everyone once approved, and to its creator always. $1 is the
// viewer's id, or NULL when anonymous. Every query that reads categories
// on behalf of a user filters with it.
const visibleTo = `(cat.status = 'approved' OR cat.created_by = $1::uuid)`

// categorySelect returns the columns scanCategory reads; $1 is the viewer.
const categorySelect = `
	SELECT cat.id, cat.slug, cat.name, cat.description, cat.attribute_schema, cat.catalog_identifier,
	       cat.status, COALESCE(cat.created_by = $1::uuid, false),
	       CASE WHEN cat.created_by = $1::uuid THEN (
	           SELECT r.decision_reason FROM moderation_requests r
	           WHERE r.kind = 'category' AND r.category_id = cat.id AND r.status <> 'pending'
	           ORDER BY r.decided_at DESC, r.id DESC LIMIT 1)
	       END
	FROM categories cat
`

// viewer maps a user id to the $1 parameter: NULL for anonymous callers or
// malformed ids, which then see approved categories only.
func viewer(userID string) any {
	if !validIDs(userID) {
		return nil
	}
	return userID
}

// List returns the categories the viewer can see, by name: every approved
// one plus the viewer's own. viewerID is "" when anonymous.
func (c *Categories) List(ctx context.Context, viewerID string) ([]Category, error) {
	rows, err := c.pool.Query(ctx,
		categorySelect+" WHERE "+visibleTo+" ORDER BY lower(cat.name), cat.slug", viewer(viewerID))
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return collectCategories(rows)
}

// Search uses the trigram index on categories.name to match a fuzzy query
// among the categories the viewer can see. Empty query falls back to List.
func (c *Categories) Search(ctx context.Context, viewerID, q string) ([]Category, error) {
	if q == "" {
		return c.List(ctx, viewerID)
	}
	rows, err := c.pool.Query(ctx,
		categorySelect+" WHERE "+visibleTo+
			" AND (cat.name ILIKE '%' || $2 || '%' OR cat.slug ILIKE '%' || $2 || '%')"+
			" ORDER BY similarity(cat.name, $2) DESC, lower(cat.name) ASC, cat.slug",
		viewer(viewerID), q,
	)
	if err != nil {
		return nil, fmt.Errorf("search categories: %w", err)
	}
	return collectCategories(rows)
}

// GetBySlug returns the category if the viewer can see it. Another user's
// unapproved category is ErrNotFound, exactly like an unknown slug.
func (c *Categories) GetBySlug(ctx context.Context, viewerID, slug string) (Category, error) {
	cat, err := scanCategory(c.pool.QueryRow(ctx,
		categorySelect+" WHERE cat.slug = $2 AND "+visibleTo, viewer(viewerID), slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, fmt.Errorf("get category %q: %w", slug, err)
	}
	return cat, nil
}

// GetByID returns any category by id, with no visibility check. Only for
// callers that already authorized access some other way: the public
// collection view, where the privacy rule let the viewer see the owner's
// collection (and so the category it is in).
func (c *Categories) GetByID(ctx context.Context, id string) (Category, error) {
	if !validIDs(id) {
		return Category{}, ErrNotFound
	}
	cat, err := scanCategory(c.pool.QueryRow(ctx, categorySelect+" WHERE cat.id = $2", nil, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, fmt.Errorf("get category by id: %w", err)
	}
	return cat, nil
}

// ListMine returns the categories the user built, whatever their status,
// newest first.
func (c *Categories) ListMine(ctx context.Context, userID string) ([]Category, error) {
	if !validIDs(userID) {
		return []Category{}, nil
	}
	rows, err := c.pool.Query(ctx,
		categorySelect+" WHERE cat.created_by = $1 ORDER BY cat.created_at DESC, cat.id DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("list my categories: %w", err)
	}
	return collectCategories(rows)
}

// ApprovedSources returns every approved category's schema: the input to
// the field library (category.Library).
func (c *Categories) ApprovedSources(ctx context.Context) ([]category.Source, error) {
	rows, err := c.pool.Query(ctx,
		`SELECT slug, name, attribute_schema FROM categories WHERE status = 'approved' ORDER BY lower(name), slug`)
	if err != nil {
		return nil, fmt.Errorf("list category schemas: %w", err)
	}
	defer rows.Close()
	out := make([]category.Source, 0)
	for rows.Next() {
		var s category.Source
		var raw []byte
		if err := rows.Scan(&s.Slug, &s.Name, &raw); err != nil {
			return nil, err
		}
		s.Schema = json.RawMessage(raw)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Create stores a new private category for the user. If the cleaned name
// matches a category the user can already see, nothing is created and a
// *CategoryExistsError names that category.
func (c *Categories) Create(ctx context.Context, userID string, in CategoryInput) (Category, error) {
	if !validIDs(userID) {
		return Category{}, ErrNotFound
	}
	in.Name = textnorm.Clean(in.Name)
	in.Description = cleanOptional(in.Description)
	for attempt := 0; ; attempt++ {
		cat, err := c.createOnce(ctx, userID, in)
		if isUniqueViolation(err, "categories_slug_key") && attempt < 3 {
			continue // a random slug suffix collided; draw another
		}
		if isUniqueViolation(err, "categories_owner_name_key") || isUniqueViolation(err, "categories_approved_name_key") {
			if existing, ok, lookErr := c.visibleByName(ctx, c.pool, userID, in.Name, ""); lookErr == nil && ok {
				return Category{}, &CategoryExistsError{Existing: existing}
			}
		}
		return cat, err
	}
}

func (c *Categories) createOnce(ctx context.Context, userID string, in CategoryInput) (Category, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return Category{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if existing, ok, err := c.visibleByName(ctx, tx, userID, in.Name, ""); err != nil {
		return Category{}, err
	} else if ok {
		return Category{}, &CategoryExistsError{Existing: existing}
	}
	suffix, err := randomSuffix()
	if err != nil {
		return Category{}, err
	}
	var id string
	if err := tx.QueryRow(ctx, `
		INSERT INTO categories (slug, name, description, attribute_schema, catalog_identifier, status, created_by)
		VALUES ($1, $2, $3, $4::jsonb, $5, 'private', $6)
		RETURNING id`,
		category.Slug(in.Name, suffix), in.Name, in.Description, string(in.Schema), in.CatalogIdentifier, userID,
	).Scan(&id); err != nil {
		return Category{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Category{}, err
	}
	return c.getOwn(ctx, userID, id)
}

// Update replaces the user's category's name, description, and schema.
// Only the creator may edit, and only while it is private or rejected
// (ErrCategoryLocked otherwise; ErrNotFound if it isn't theirs). An edit
// may not remove a field, or an option, that the creator's items still
// use (*SchemaInUseError). Editing a rejected category makes it private
// again, ready to resubmit.
func (c *Categories) Update(ctx context.Context, userID, slug string, in CategoryInput) (Category, error) {
	if !validIDs(userID) {
		return Category{}, ErrNotFound
	}
	in.Name = textnorm.Clean(in.Name)
	in.Description = cleanOptional(in.Description)

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return Category{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id, status, nameKey string
	var current []byte
	err = tx.QueryRow(ctx, `
		SELECT id, status, name_key, attribute_schema FROM categories
		WHERE slug = $1 AND created_by = $2 FOR UPDATE`, slug, userID).Scan(&id, &status, &nameKey, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, fmt.Errorf("lock category: %w", err)
	}
	if status != CategoryPrivate && status != CategoryRejected {
		return Category{}, ErrCategoryLocked
	}
	// Only a rename is checked against the names you can see: a category
	// whose name a later-approved one took must still be editable. The
	// unique indexes catch any real clash.
	if textnorm.Key(in.Name) != nameKey {
		if existing, ok, err := c.visibleByName(ctx, tx, userID, in.Name, id); err != nil {
			return Category{}, err
		} else if ok {
			return Category{}, &CategoryExistsError{Existing: existing}
		}
	}
	if err := checkSchemaInUse(ctx, tx, id, current, in.Schema); err != nil {
		return Category{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE categories
		SET name = $2, description = $3, attribute_schema = $4::jsonb, catalog_identifier = $5,
		    status = 'private', updated_at = now()
		WHERE id = $1`,
		id, in.Name, in.Description, string(in.Schema), in.CatalogIdentifier); err != nil {
		if isUniqueViolation(err, "categories_owner_name_key") || isUniqueViolation(err, "categories_approved_name_key") {
			if existing, ok, lookErr := c.visibleByName(ctx, c.pool, userID, in.Name, id); lookErr == nil && ok {
				return Category{}, &CategoryExistsError{Existing: existing}
			}
		}
		return Category{}, fmt.Errorf("update category: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Category{}, err
	}
	return c.getOwn(ctx, userID, id)
}

func (c *Categories) getOwn(ctx context.Context, userID, id string) (Category, error) {
	return scanCategory(c.pool.QueryRow(ctx, categorySelect+" WHERE cat.id = $2 AND cat.created_by = $1", userID, id))
}

// visibleByName finds a category the user can see whose name matches
// name case- and space-insensitively, preferring an approved one. exceptID
// ("" for none) is left out, so a category doesn't clash with itself.
func (c *Categories) visibleByName(ctx context.Context, q querier, userID, name, exceptID string) (Category, bool, error) {
	var except any
	if exceptID != "" {
		except = exceptID
	}
	cat, err := scanCategory(q.QueryRow(ctx, categorySelect+`
		WHERE cat.name_key = $2 AND `+visibleTo+` AND ($3::uuid IS NULL OR cat.id <> $3::uuid)
		ORDER BY (cat.status = 'approved') DESC
		LIMIT 1`, userID, textnorm.Key(name), except))
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, false, nil
	}
	if err != nil {
		return Category{}, false, fmt.Errorf("find category by name: %w", err)
	}
	return cat, true, nil
}

// checkSchemaInUse refuses a schema change that would leave the category's
// items with values the new schema rejects: a removed (or retyped) field
// some item has a value for, or a list of options that leaves out a value
// some item uses.
func checkSchemaInUse(ctx context.Context, tx pgx.Tx, categoryID string, oldRaw, newRaw []byte) error {
	type prop struct {
		Type  string   `json:"type"`
		Title string   `json:"title"`
		Enum  []string `json:"enum"`
	}
	var oldS, newS struct {
		Properties map[string]prop `json:"properties"`
	}
	if err := json.Unmarshal(oldRaw, &oldS); err != nil {
		return fmt.Errorf("decode current schema: %w", err)
	}
	if err := json.Unmarshal(newRaw, &newS); err != nil {
		return fmt.Errorf("decode new schema: %w", err)
	}
	keys := make([]string, 0, len(oldS.Properties))
	for k := range oldS.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		old := oldS.Properties[key]
		title := old.Title
		if title == "" {
			title = key
		}
		next, kept := newS.Properties[key]
		var n int
		switch {
		case !kept || next.Type != old.Type:
			if err := tx.QueryRow(ctx, `
				SELECT count(*) FROM items i JOIN collections c ON c.id = i.collection_id
				WHERE c.category_id = $1 AND i.attributes ? $2`, categoryID, key).Scan(&n); err != nil {
				return fmt.Errorf("count field use: %w", err)
			}
			if n > 0 {
				return &SchemaInUseError{Message: fmt.Sprintf(
					"%s %s a value for %q. Clear it from %s first, or keep the field.",
					countItems(n), hasHave(n), title, itThem(n))}
			}
		case len(next.Enum) > 0:
			var example string
			if err := tx.QueryRow(ctx, `
				SELECT count(*), COALESCE(min(i.attributes ->> $2), '')
				FROM items i JOIN collections c ON c.id = i.collection_id
				WHERE c.category_id = $1 AND i.attributes ? $2
				  AND NOT (i.attributes ->> $2 = ANY ($3::text[]))`, categoryID, key, next.Enum).Scan(&n, &example); err != nil {
				return fmt.Errorf("count option use: %w", err)
			}
			if n > 0 {
				return &SchemaInUseError{Message: fmt.Sprintf(
					"%s %s %q as %q, which isn't in the new list. Keep that option, or change %s first.",
					countItems(n), useUses(n), example, title, itThem(n))}
			}
		}
	}
	return nil
}

func countItems(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

func hasHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

func useUses(n int) string {
	if n == 1 {
		return "uses"
	}
	return "use"
}

func itThem(n int) string {
	if n == 1 {
		return "it"
	}
	return "them"
}

func randomSuffix() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random slug suffix: %w", err)
	}
	return strings.ToLower(hex.EncodeToString(b)), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCategory(r rowScanner) (Category, error) {
	var cat Category
	var raw []byte
	if err := r.Scan(&cat.ID, &cat.Slug, &cat.Name, &cat.Description, &raw, &cat.CatalogIdentifier,
		&cat.Status, &cat.Mine, &cat.ReviewReason); err != nil {
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
