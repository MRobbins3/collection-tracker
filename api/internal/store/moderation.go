package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// Moderation errors. ErrNotFound covers unknown requests, entries, and
// items that aren't the caller's.
var (
	// ErrAlreadyDecided: another admin decided the request first.
	ErrAlreadyDecided = errors.New("store: request already decided")
	// ErrDuplicateEntry: the change would give the entry the same name and
	// identifier as another live entry in its category (ADR 0011).
	ErrDuplicateEntry = errors.New("store: duplicate catalog entry")
	// ErrMergeTarget: the merge target isn't an approved entry in the
	// submission's category.
	ErrMergeTarget = errors.New("store: merge target unavailable")
	// ErrWrongKind: the action doesn't apply to this kind of request.
	ErrWrongKind = errors.New("store: action doesn't apply to this request")
	// ErrSubmissionLimit: the user already has MaxPendingPerUser requests
	// waiting for review.
	ErrSubmissionLimit = errors.New("store: too many pending submissions")
	// ErrNoChanges: a correction that changes nothing.
	ErrNoChanges = errors.New("store: no changes proposed")
	// ErrEntryNotPending: a submission's entry is no longer pending although
	// its request is (changed outside the queue), so the decision is refused
	// rather than leaving the request and the entry disagreeing.
	ErrEntryNotPending = errors.New("store: submitted entry is no longer pending")
)

// MaxPendingPerUser caps how many requests (submissions plus corrections)
// one user can have waiting at once, so a single account can't flood the
// queue. Deciding one frees a slot. ADR 0012.
const MaxPendingPerUser = 20

// mySubmissionsLimit bounds "My submissions"; with the pending cap, a user
// would need years of activity to pass it.
const mySubmissionsLimit = 200

// ModerationKind is what a request asks admins to review. New kinds (such
// as user-built categories) add a value here and in the table's CHECK.
type ModerationKind string

const (
	KindCatalogEntry      ModerationKind = "catalog_entry"
	KindCatalogCorrection ModerationKind = "catalog_correction"
)

// ModerationStatus is a request's outcome so far.
type ModerationStatus string

const (
	StatusPending  ModerationStatus = "pending"
	StatusApproved ModerationStatus = "approved"
	StatusRejected ModerationStatus = "rejected"
	StatusMerged   ModerationStatus = "merged"
)

// EntryFields is a set of catalog entry fields: a correction's proposed
// changes, an admin's edits, and the audit record of what changed. Nil
// means "not included". An empty Description clears it.
type EntryFields struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Attributes  json.RawMessage `json:"attributes,omitempty"`
}

func (f EntryFields) empty() bool {
	return f.Name == nil && f.Description == nil && f.Attributes == nil
}

// SubmissionInput is a user's proposed new catalog entry. Attributes must
// already be validated against the category schema (the handler does it).
type SubmissionInput struct {
	CategorySlug string
	Name         string
	Description  *string
	Attributes   json.RawMessage
	// SourceItemID is the user's free-form item this came from. It is
	// linked to the entry when the entry is approved (or already exists).
	SourceItemID *string
}

// SubmitOutcome says what a submission did.
type SubmitOutcome string

const (
	// OutcomeSubmitted: a new pending entry is waiting for review.
	OutcomeSubmitted SubmitOutcome = "submitted"
	// OutcomeAlreadyInCatalog: an approved entry has the same name and
	// identifier; nothing new was created (the source item is linked to it).
	OutcomeAlreadyInCatalog SubmitOutcome = "already_in_catalog"
	// OutcomeAlreadySubmitted: you already submitted this; it is still
	// waiting.
	OutcomeAlreadySubmitted SubmitOutcome = "already_submitted"
	// OutcomePendingReview: someone else's identical submission is
	// waiting. Its details stay private.
	OutcomePendingReview SubmitOutcome = "pending_review"
)

// SubmitResult is the outcome plus whatever the caller may see.
type SubmitResult struct {
	Outcome SubmitOutcome `json:"outcome"`
	// Entry is the approved match (OutcomeAlreadyInCatalog only).
	Entry *CatalogEntry `json:"entry,omitempty"`
	// Submission is your request (submitted / already submitted).
	Submission *Submission `json:"submission,omitempty"`
	// ItemLinked: the source item was linked to Entry.
	ItemLinked bool `json:"item_linked"`
}

// Submission is the submitter's view of one of their own requests.
type Submission struct {
	ID           string           `json:"id"`
	Kind         ModerationKind   `json:"kind"`
	Status       ModerationStatus `json:"status"`
	CategorySlug string           `json:"category_slug"`
	CategoryName string           `json:"category_name"`
	// Name and Identifier are the submitted entry's, or for a correction
	// the entry being corrected.
	Name       string          `json:"name"`
	Identifier *string         `json:"identifier,omitempty"`
	Attributes json.RawMessage `json:"attributes"`
	Proposed   *EntryFields    `json:"proposed,omitempty"`
	Note       *string         `json:"note,omitempty"`
	// Reason is the admin's explanation, shown for rejections and merges.
	Reason    *string    `json:"reason,omitempty"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	// EntryID is the public catalog entry this led to: the approved
	// submission, the corrected entry, or the entry it was merged into.
	// Nil while a submission is pending or after it was rejected.
	EntryID        *string `json:"entry_id,omitempty"`
	MergedIntoName *string `json:"merged_into_name,omitempty"`
}

// UserRef names a person on a request without exposing their id or email.
type UserRef struct {
	DisplayName string `json:"display_name"`
	Handle      string `json:"handle"`
}

// QueueQuery selects one page of pending requests. Empty Kind or
// CategorySlug means "any".
type QueueQuery struct {
	Kind         ModerationKind
	CategorySlug string
	Limit        int
	Offset       int
}

// QueueItem is one row of the admin queue.
type QueueItem struct {
	ID           string         `json:"id"`
	Kind         ModerationKind `json:"kind"`
	CategorySlug string         `json:"category_slug"`
	CategoryName string         `json:"category_name"`
	Name         string         `json:"name"`
	Identifier   *string        `json:"identifier,omitempty"`
	Note         *string        `json:"note,omitempty"`
	Submitter    UserRef        `json:"submitter"`
	CreatedAt    time.Time      `json:"created_at"`
}

// QueuePage is one page of the queue plus the total pending count.
type QueuePage struct {
	Requests []QueueItem `json:"requests"`
	Total    int         `json:"total"`
	HasMore  bool        `json:"has_more"`
}

// ModerationRequest is the admin's full view of one request.
type ModerationRequest struct {
	ID           string           `json:"id"`
	Kind         ModerationKind   `json:"kind"`
	Status       ModerationStatus `json:"status"`
	CategorySlug string           `json:"category_slug"`
	CategoryName string           `json:"category_name"`
	// Entry is the submitted entry (any status) or, for a correction, the
	// entry being corrected as it is now.
	Entry     CatalogEntry `json:"entry"`
	Proposed  *EntryFields `json:"proposed,omitempty"`
	Previous  *EntryFields `json:"previous,omitempty"`
	Applied   *EntryFields `json:"applied,omitempty"`
	Note      *string      `json:"note,omitempty"`
	Submitter UserRef      `json:"submitter"`
	DecidedBy *UserRef     `json:"decided_by,omitempty"`
	DecidedAt *time.Time   `json:"decided_at,omitempty"`
	Reason    *string      `json:"reason,omitempty"`
	// MergedInto is the existing entry a duplicate was merged into.
	MergedInto *CatalogEntry `json:"merged_into,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

// Moderation owns user submissions, corrections, and the admin queue
// (ADR 0012). Every decision is a conditional update on a locked pending
// request, so two admins can't both decide the same one.
type Moderation struct {
	pool *pgxpool.Pool
}

func NewModeration(pool *pgxpool.Pool) *Moderation {
	return &Moderation{pool: pool}
}

// querier is satisfied by both the pool and a transaction.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Submit records a user's proposed new catalog entry. The name is cleaned
// (textnorm) and compared, with the identifier, against live entries in
// the category (ADR 0011): an approved match is reused, a pending match
// is reported, and only a genuinely new entry is queued.
func (m *Moderation) Submit(ctx context.Context, userID string, in SubmissionInput) (SubmitResult, error) {
	if !validIDs(userID) || (in.SourceItemID != nil && !validIDs(*in.SourceItemID)) {
		return SubmitResult{}, ErrNotFound
	}
	attrs, err := cleanAttributes(in.Attributes)
	if err != nil {
		return SubmitResult{}, err
	}
	in.Name = textnorm.Clean(in.Name)
	in.Description = cleanOptional(in.Description)
	in.Attributes = attrs

	// A concurrent identical submission can win the natural key between
	// our lookup and insert; the retry then finds it.
	for attempt := 0; ; attempt++ {
		res, err := m.submitOnce(ctx, userID, in)
		if isUniqueViolation(err, "catalog_entries_natural_key") && attempt == 0 {
			continue
		}
		return res, err
	}
}

func (m *Moderation) submitOnce(ctx context.Context, userID string, in SubmissionInput) (SubmitResult, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return SubmitResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUser(ctx, tx, userID); err != nil {
		return SubmitResult{}, err
	}

	var categoryID string
	var identifier *string
	err = tx.QueryRow(ctx,
		`SELECT id, $2::jsonb ->> catalog_identifier FROM categories WHERE slug = $1`,
		in.CategorySlug, string(in.Attributes)).Scan(&categoryID, &identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return SubmitResult{}, ErrNotFound
	}
	if err != nil {
		return SubmitResult{}, fmt.Errorf("lookup category: %w", err)
	}

	if in.SourceItemID != nil {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM items i JOIN collections c ON c.id = i.collection_id
				WHERE i.id = $1 AND c.user_id = $2 AND c.category_id = $3
			)`, *in.SourceItemID, userID, categoryID).Scan(&exists); err != nil {
			return SubmitResult{}, fmt.Errorf("check source item: %w", err)
		}
		if !exists {
			return SubmitResult{}, ErrNotFound
		}
	}

	identifierKey := ""
	if identifier != nil {
		identifierKey = textnorm.Key(*identifier)
	}
	var matchID, matchStatus string
	var matchSubmitter *string
	err = tx.QueryRow(ctx, `
		SELECT id, status, submitted_by FROM catalog_entries
		WHERE category_id = $1 AND status IN ('pending', 'approved')
		  AND name_key = $2 AND identifier_key = $3`,
		categoryID, textnorm.Key(in.Name), identifierKey).Scan(&matchID, &matchStatus, &matchSubmitter)
	switch {
	case err == nil && matchStatus == "approved":
		linked, err := linkFreeFormItem(ctx, tx, in.SourceItemID, matchID)
		if err != nil {
			return SubmitResult{}, err
		}
		entry, err := scanCatalogEntry(tx.QueryRow(ctx, entryByIDSQL, matchID))
		if err != nil {
			return SubmitResult{}, fmt.Errorf("load match: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return SubmitResult{}, err
		}
		return SubmitResult{Outcome: OutcomeAlreadyInCatalog, Entry: &entry, ItemLinked: linked}, nil
	case err == nil && matchSubmitter != nil && *matchSubmitter == userID:
		var requestID string
		if err := tx.QueryRow(ctx,
			`SELECT id FROM moderation_requests WHERE catalog_entry_id = $1 AND kind = 'catalog_entry'`,
			matchID).Scan(&requestID); err != nil {
			return SubmitResult{}, fmt.Errorf("load own submission: %w", err)
		}
		sub, err := getSubmission(ctx, tx, userID, requestID)
		if err != nil {
			return SubmitResult{}, err
		}
		return SubmitResult{Outcome: OutcomeAlreadySubmitted, Submission: &sub}, nil
	case err == nil:
		return SubmitResult{Outcome: OutcomePendingReview}, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return SubmitResult{}, fmt.Errorf("find duplicate: %w", err)
	}

	if err := checkPendingCap(ctx, tx, userID); err != nil {
		return SubmitResult{}, err
	}
	var entryID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO catalog_entries (category_id, name, identifier, description, attributes, source, status, submitted_by)
		VALUES ($1, $2, $3, $4, $5::jsonb, 'user_submitted', 'pending', $6)
		RETURNING id`,
		categoryID, in.Name, identifier, in.Description, string(in.Attributes), userID).Scan(&entryID); err != nil {
		return SubmitResult{}, err
	}
	var requestID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO moderation_requests (kind, category_id, catalog_entry_id, submitted_by, source_item_id)
		VALUES ('catalog_entry', $1, $2, $3, $4)
		RETURNING id`, categoryID, entryID, userID, in.SourceItemID).Scan(&requestID); err != nil {
		return SubmitResult{}, fmt.Errorf("insert request: %w", err)
	}
	sub, err := getSubmission(ctx, tx, userID, requestID)
	if err != nil {
		return SubmitResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{Outcome: OutcomeSubmitted, Submission: &sub}, nil
}

// ProposeCorrection records proposed changes to an approved entry. Only
// the fields that actually differ are kept (ErrNoChanges if none); the
// entry itself is untouched until an admin approves. Attributes must be
// validated by the caller.
func (m *Moderation) ProposeCorrection(ctx context.Context, userID, entryID string, proposed EntryFields, note *string) (Submission, error) {
	if !validIDs(userID, entryID) {
		return Submission{}, ErrNotFound
	}
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return Submission{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUser(ctx, tx, userID); err != nil {
		return Submission{}, err
	}
	var categoryID string
	var cur entryValues
	err = tx.QueryRow(ctx, `
		SELECT category_id, name, description, attributes FROM catalog_entries
		WHERE id = $1 AND status = 'approved'`, entryID).Scan(&categoryID, &cur.Name, &cur.Description, &cur.Attributes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	if err != nil {
		return Submission{}, fmt.Errorf("load entry: %w", err)
	}
	changes, err := changedFields(cur, proposed)
	if err != nil {
		return Submission{}, err
	}
	if changes.empty() {
		return Submission{}, ErrNoChanges
	}
	if err := checkPendingCap(ctx, tx, userID); err != nil {
		return Submission{}, err
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return Submission{}, err
	}
	var requestID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO moderation_requests (kind, category_id, catalog_entry_id, submitted_by, proposed, note)
		VALUES ('catalog_correction', $1, $2, $3, $4::jsonb, $5)
		RETURNING id`, categoryID, entryID, userID, string(raw), cleanOptional(note)).Scan(&requestID); err != nil {
		return Submission{}, fmt.Errorf("insert correction: %w", err)
	}
	sub, err := getSubmission(ctx, tx, userID, requestID)
	if err != nil {
		return Submission{}, err
	}
	return sub, tx.Commit(ctx)
}

// ListMine returns the user's own requests, newest first, with outcomes.
func (m *Moderation) ListMine(ctx context.Context, userID string) ([]Submission, error) {
	if !validIDs(userID) {
		return []Submission{}, nil
	}
	rows, err := m.pool.Query(ctx, submissionSQL+`
		WHERE r.submitted_by = $1
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2`, userID, mySubmissionsLimit)
	if err != nil {
		return nil, fmt.Errorf("list my submissions: %w", err)
	}
	defer rows.Close()
	out := make([]Submission, 0)
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Queue returns one page of pending requests, oldest first (id breaks
// ties so pages never skip or repeat a row).
func (m *Moderation) Queue(ctx context.Context, q QueueQuery) (QueuePage, error) {
	const where = `
		WHERE r.status = 'pending'
		  AND ($1 = '' OR r.kind = $1)
		  AND ($2 = '' OR cat.slug = $2)
	`
	const from = `
		FROM moderation_requests r
		JOIN categories cat ON cat.id = r.category_id
		JOIN catalog_entries ce ON ce.id = r.catalog_entry_id
		JOIN users su ON su.id = r.submitted_by
	`
	page := QueuePage{Requests: make([]QueueItem, 0)}
	if err := m.pool.QueryRow(ctx, `SELECT count(*)`+from+where, string(q.Kind), q.CategorySlug).Scan(&page.Total); err != nil {
		return QueuePage{}, fmt.Errorf("count queue: %w", err)
	}
	rows, err := m.pool.Query(ctx, `
		SELECT r.id, r.kind, cat.slug, cat.name, ce.name, ce.identifier, r.note,
		       su.display_name, su.handle, r.created_at
		`+from+where+`
		ORDER BY r.created_at, r.id
		LIMIT $3 OFFSET $4`, string(q.Kind), q.CategorySlug, q.Limit+1, q.Offset)
	if err != nil {
		return QueuePage{}, fmt.Errorf("list queue: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it QueueItem
		if err := rows.Scan(&it.ID, &it.Kind, &it.CategorySlug, &it.CategoryName, &it.Name, &it.Identifier,
			&it.Note, &it.Submitter.DisplayName, &it.Submitter.Handle, &it.CreatedAt); err != nil {
			return QueuePage{}, err
		}
		page.Requests = append(page.Requests, it)
	}
	if err := rows.Err(); err != nil {
		return QueuePage{}, err
	}
	if len(page.Requests) > q.Limit {
		page.Requests = page.Requests[:q.Limit]
		page.HasMore = true
	}
	return page, nil
}

// Get returns one request in full, whatever its status.
func (m *Moderation) Get(ctx context.Context, id string) (ModerationRequest, error) {
	if !validIDs(id) {
		return ModerationRequest{}, ErrNotFound
	}
	return getRequest(ctx, m.pool, id)
}

// Approve accepts a pending request. A submission is published
// (status approved, approved_by/approved_at set) and its source item
// linked; a correction is applied to its entry. edits, when given,
// replace the submitted values first (edit-then-approve); the audit
// columns keep both. ErrAlreadyDecided if someone decided it first;
// ErrDuplicateEntry if the result would clash with another entry.
func (m *Moderation) Approve(ctx context.Context, adminID, id string, edits *EntryFields) (ModerationRequest, error) {
	if !validIDs(adminID, id) {
		return ModerationRequest{}, ErrNotFound
	}
	err := m.decide(ctx, id, func(tx pgx.Tx, req lockedRequest) (decision, error) {
		switch req.Kind {
		case KindCatalogEntry:
			return approveEntry(ctx, tx, adminID, req, edits)
		case KindCatalogCorrection:
			return applyCorrection(ctx, tx, req, edits)
		}
		return decision{}, ErrWrongKind
	}, StatusApproved, adminID)
	if err != nil {
		return ModerationRequest{}, err
	}
	return m.Get(ctx, id)
}

// Reject declines a pending request with a reason the submitter sees. A
// rejected submission stays private (status rejected); a rejected
// correction leaves its entry untouched.
func (m *Moderation) Reject(ctx context.Context, adminID, id, reason string) (ModerationRequest, error) {
	if !validIDs(adminID, id) {
		return ModerationRequest{}, ErrNotFound
	}
	err := m.decide(ctx, id, func(tx pgx.Tx, req lockedRequest) (decision, error) {
		if req.Kind == KindCatalogEntry {
			if err := retirePendingEntry(ctx, tx, req.EntryID); err != nil {
				return decision{}, err
			}
		}
		return decision{reason: &reason}, nil
	}, StatusRejected, adminID)
	if err != nil {
		return ModerationRequest{}, err
	}
	return m.Get(ctx, id)
}

// Merge resolves a duplicate submission into an existing approved entry
// in the same category: the submission never goes public (its entry
// becomes rejected, freeing the name), its source item is linked to the
// existing entry, and the submitter sees which entry it became.
func (m *Moderation) Merge(ctx context.Context, adminID, id, targetID string, reason *string) (ModerationRequest, error) {
	if !validIDs(adminID, id) {
		return ModerationRequest{}, ErrNotFound
	}
	err := m.decide(ctx, id, func(tx pgx.Tx, req lockedRequest) (decision, error) {
		if req.Kind != KindCatalogEntry {
			return decision{}, ErrWrongKind
		}
		if !validIDs(targetID) {
			return decision{}, ErrMergeTarget
		}
		var ok bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM catalog_entries
				WHERE id = $1 AND category_id = $2 AND status = 'approved'
			)`, targetID, req.CategoryID).Scan(&ok); err != nil {
			return decision{}, fmt.Errorf("check merge target: %w", err)
		}
		if !ok {
			return decision{}, ErrMergeTarget
		}
		if err := retirePendingEntry(ctx, tx, req.EntryID); err != nil {
			return decision{}, err
		}
		if _, err := linkSourceItem(ctx, tx, req, targetID); err != nil {
			return decision{}, err
		}
		return decision{reason: cleanOptional(reason), mergedInto: &targetID}, nil
	}, StatusMerged, adminID)
	if err != nil {
		return ModerationRequest{}, err
	}
	return m.Get(ctx, id)
}

// ---- decisions --------------------------------------------------------------

// lockedRequest is a pending request row held FOR UPDATE.
type lockedRequest struct {
	Kind         ModerationKind
	CategoryID   string
	EntryID      string
	SubmittedBy  string
	SourceItemID *string
	Proposed     *EntryFields
}

// decision is what an action wants recorded on the request.
type decision struct {
	reason     *string
	mergedInto *string
	previous   *EntryFields
	applied    *EntryFields
}

// decide locks the request, refuses it unless pending, runs act, and
// records the outcome, all in one transaction. The row lock makes a
// second admin wait; once the first commits, the second sees a decided
// request and gets ErrAlreadyDecided. The final UPDATE is conditional on
// status = 'pending' as well, so the guard doesn't rest on the lock alone.
func (m *Moderation) decide(ctx context.Context, id string, act func(pgx.Tx, lockedRequest) (decision, error), outcome ModerationStatus, adminID string) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var req lockedRequest
	var status ModerationStatus
	var proposed []byte
	err = tx.QueryRow(ctx, `
		SELECT kind, status, category_id, catalog_entry_id, submitted_by, source_item_id, proposed
		FROM moderation_requests WHERE id = $1 FOR UPDATE`, id).
		Scan(&req.Kind, &status, &req.CategoryID, &req.EntryID, &req.SubmittedBy, &req.SourceItemID, &proposed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock request: %w", err)
	}
	if status != StatusPending {
		return ErrAlreadyDecided
	}
	if proposed != nil {
		req.Proposed = &EntryFields{}
		if err := json.Unmarshal(proposed, req.Proposed); err != nil {
			return fmt.Errorf("decode proposal: %w", err)
		}
	}

	d, err := act(tx, req)
	if err != nil {
		if isUniqueViolation(err, "catalog_entries_natural_key") {
			return ErrDuplicateEntry
		}
		return err
	}
	previous, err := jsonOrNil(d.previous)
	if err != nil {
		return err
	}
	applied, err := jsonOrNil(d.applied)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE moderation_requests
		SET status = $2, decided_by = $3, decided_at = now(), decision_reason = $4,
		    merged_into_id = $5, previous = $6::jsonb, applied = $7::jsonb, updated_at = now()
		WHERE id = $1 AND status = 'pending'`,
		id, string(outcome), adminID, d.reason, d.mergedInto, previous, applied)
	if err != nil {
		return fmt.Errorf("record decision: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyDecided
	}
	return tx.Commit(ctx)
}

func approveEntry(ctx context.Context, tx pgx.Tx, adminID string, req lockedRequest, edits *EntryFields) (decision, error) {
	cur, err := loadEntryValues(ctx, tx, req.EntryID)
	if err != nil {
		return decision{}, err
	}
	var d decision
	final := cur
	if edits != nil {
		changes, err := changedFields(cur, *edits)
		if err != nil {
			return decision{}, err
		}
		if !changes.empty() {
			prev := cur.fieldsFor(changes)
			d.previous, d.applied = &prev, &changes
			final = cur.with(changes)
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE catalog_entries ce
		SET name = $2, description = $3, attributes = $4::jsonb,
		    identifier = $4::jsonb ->> cat.catalog_identifier,
		    status = 'approved', approved_by = $5, approved_at = now(), updated_at = now()
		FROM categories cat
		WHERE ce.id = $1 AND cat.id = ce.category_id AND ce.status = 'pending'`,
		req.EntryID, final.Name, final.Description, string(final.Attributes), adminID)
	if err != nil {
		return decision{}, err
	}
	if tag.RowsAffected() == 0 {
		return decision{}, ErrEntryNotPending
	}
	if _, err := linkSourceItem(ctx, tx, req, req.EntryID); err != nil {
		return decision{}, err
	}
	return d, nil
}

func applyCorrection(ctx context.Context, tx pgx.Tx, req lockedRequest, edits *EntryFields) (decision, error) {
	var cur entryValues
	err := tx.QueryRow(ctx, `
		SELECT name, description, attributes FROM catalog_entries
		WHERE id = $1 AND status = 'approved' FOR UPDATE`, req.EntryID).Scan(&cur.Name, &cur.Description, &cur.Attributes)
	if errors.Is(err, pgx.ErrNoRows) {
		return decision{}, ErrNotFound
	}
	if err != nil {
		return decision{}, fmt.Errorf("lock entry: %w", err)
	}
	proposal := EntryFields{}
	if req.Proposed != nil {
		proposal = *req.Proposed
	}
	if edits != nil {
		proposal = *edits
	}
	changes, err := changedFields(cur, proposal)
	if err != nil {
		return decision{}, err
	}
	if changes.empty() {
		// Nothing left to change (the entry already says this): approve as
		// a no-op so the submitter still hears back.
		return decision{}, nil
	}
	final := cur.with(changes)
	if _, err := tx.Exec(ctx, `
		UPDATE catalog_entries ce
		SET name = $2, description = $3, attributes = $4::jsonb,
		    identifier = $4::jsonb ->> cat.catalog_identifier, updated_at = now()
		FROM categories cat
		WHERE ce.id = $1 AND cat.id = ce.category_id`,
		req.EntryID, final.Name, final.Description, string(final.Attributes)); err != nil {
		return decision{}, err
	}
	prev := cur.fieldsFor(changes)
	return decision{previous: &prev, applied: &changes}, nil
}

// retirePendingEntry marks a submission's pending entry rejected (reject,
// or merge into another entry). ErrEntryNotPending if it wasn't pending.
func retirePendingEntry(ctx context.Context, tx pgx.Tx, entryID string) error {
	tag, err := tx.Exec(ctx, `
		UPDATE catalog_entries SET status = 'rejected', updated_at = now()
		WHERE id = $1 AND status = 'pending'`, entryID)
	if err != nil {
		return fmt.Errorf("retire entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEntryNotPending
	}
	return nil
}

// linkSourceItem links the request's source item to entryID if it is
// still the submitter's, still free-form, and in the entry's category.
func linkSourceItem(ctx context.Context, tx pgx.Tx, req lockedRequest, entryID string) (bool, error) {
	if req.SourceItemID == nil {
		return false, nil
	}
	tag, err := tx.Exec(ctx, `
		UPDATE items i SET catalog_entry_id = $1, updated_at = now()
		FROM collections c
		WHERE i.id = $2 AND c.id = i.collection_id AND c.user_id = $3 AND c.category_id = $4
		  AND i.catalog_entry_id IS NULL`,
		entryID, *req.SourceItemID, req.SubmittedBy, req.CategoryID)
	if err != nil {
		return false, fmt.Errorf("link source item: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// linkFreeFormItem links an item whose ownership and category the caller
// already checked, unless it is linked already.
func linkFreeFormItem(ctx context.Context, tx pgx.Tx, itemID *string, entryID string) (bool, error) {
	if itemID == nil {
		return false, nil
	}
	tag, err := tx.Exec(ctx, `
		UPDATE items SET catalog_entry_id = $1, updated_at = now()
		WHERE id = $2 AND catalog_entry_id IS NULL`, entryID, *itemID)
	if err != nil {
		return false, fmt.Errorf("link item: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ---- reads --------------------------------------------------------------

const entryByIDSQL = `
	SELECT ` + catalogSelectCols + `
	FROM catalog_entries ce
	JOIN categories cat ON cat.id = ce.category_id
	WHERE ce.id = $1
`

// submissionSQL selects a Submission; callers append WHERE/ORDER. The
// public entry id is the approved entry itself (or, for a correction, its
// target), or for a merge the approved entry it was merged into.
const submissionSQL = `
	SELECT r.id, r.kind, r.status, cat.slug, cat.name, ce.name, ce.identifier, ce.attributes,
	       r.proposed, r.note, r.decision_reason, r.decided_at, r.created_at,
	       CASE WHEN r.status = 'merged' THEN mi.id
	            WHEN ce.status = 'approved' THEN ce.id END,
	       mi.name
	FROM moderation_requests r
	JOIN categories cat ON cat.id = r.category_id
	JOIN catalog_entries ce ON ce.id = r.catalog_entry_id
	LEFT JOIN catalog_entries mi ON mi.id = r.merged_into_id AND mi.status = 'approved'
`

func getSubmission(ctx context.Context, q querier, userID, id string) (Submission, error) {
	s, err := scanSubmission(q.QueryRow(ctx, submissionSQL+` WHERE r.submitted_by = $1 AND r.id = $2`, userID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Submission{}, ErrNotFound
	}
	return s, err
}

func scanSubmission(r rowScanner) (Submission, error) {
	var s Submission
	var attrs, proposed []byte
	if err := r.Scan(&s.ID, &s.Kind, &s.Status, &s.CategorySlug, &s.CategoryName, &s.Name, &s.Identifier,
		&attrs, &proposed, &s.Note, &s.Reason, &s.DecidedAt, &s.CreatedAt, &s.EntryID, &s.MergedIntoName); err != nil {
		return Submission{}, err
	}
	s.Attributes = json.RawMessage(attrs)
	if proposed != nil {
		s.Proposed = &EntryFields{}
		if err := json.Unmarshal(proposed, s.Proposed); err != nil {
			return Submission{}, fmt.Errorf("decode proposal: %w", err)
		}
	}
	return s, nil
}

func getRequest(ctx context.Context, q querier, id string) (ModerationRequest, error) {
	var r ModerationRequest
	var entryID string
	var mergedInto *string
	var proposed, previous, applied []byte
	var deciderName, deciderHandle *string
	err := q.QueryRow(ctx, `
		SELECT r.id, r.kind, r.status, cat.slug, cat.name, r.catalog_entry_id, r.merged_into_id,
		       r.proposed, r.previous, r.applied, r.note, r.decision_reason, r.decided_at, r.created_at,
		       su.display_name, su.handle, du.display_name, du.handle
		FROM moderation_requests r
		JOIN categories cat ON cat.id = r.category_id
		JOIN users su ON su.id = r.submitted_by
		LEFT JOIN users du ON du.id = r.decided_by
		WHERE r.id = $1`, id).Scan(
		&r.ID, &r.Kind, &r.Status, &r.CategorySlug, &r.CategoryName, &entryID, &mergedInto,
		&proposed, &previous, &applied, &r.Note, &r.Reason, &r.DecidedAt, &r.CreatedAt,
		&r.Submitter.DisplayName, &r.Submitter.Handle, &deciderName, &deciderHandle)
	if errors.Is(err, pgx.ErrNoRows) {
		return ModerationRequest{}, ErrNotFound
	}
	if err != nil {
		return ModerationRequest{}, fmt.Errorf("get request: %w", err)
	}
	if deciderName != nil {
		r.DecidedBy = &UserRef{DisplayName: *deciderName, Handle: *deciderHandle}
	}
	for _, f := range []struct {
		raw []byte
		dst **EntryFields
	}{{proposed, &r.Proposed}, {previous, &r.Previous}, {applied, &r.Applied}} {
		if f.raw == nil {
			continue
		}
		*f.dst = &EntryFields{}
		if err := json.Unmarshal(f.raw, *f.dst); err != nil {
			return ModerationRequest{}, fmt.Errorf("decode request fields: %w", err)
		}
	}
	if r.Entry, err = scanCatalogEntry(q.QueryRow(ctx, entryByIDSQL, entryID)); err != nil {
		return ModerationRequest{}, fmt.Errorf("load request entry: %w", err)
	}
	if mergedInto != nil {
		e, err := scanCatalogEntry(q.QueryRow(ctx, entryByIDSQL, *mergedInto))
		if err != nil {
			return ModerationRequest{}, fmt.Errorf("load merge target: %w", err)
		}
		r.MergedInto = &e
	}
	return r, nil
}

// ---- helpers ------------------------------------------------------------

// entryValues are the editable fields of a catalog entry.
type entryValues struct {
	Name        string
	Description *string
	Attributes  json.RawMessage
}

func loadEntryValues(ctx context.Context, tx pgx.Tx, id string) (entryValues, error) {
	var v entryValues
	if err := tx.QueryRow(ctx, `SELECT name, description, attributes FROM catalog_entries WHERE id = $1`, id).
		Scan(&v.Name, &v.Description, &v.Attributes); err != nil {
		return entryValues{}, fmt.Errorf("load entry: %w", err)
	}
	return v, nil
}

// with returns v with the given changes applied ("" description clears).
func (v entryValues) with(c EntryFields) entryValues {
	if c.Name != nil {
		v.Name = *c.Name
	}
	if c.Description != nil {
		if *c.Description == "" {
			v.Description = nil
		} else {
			d := *c.Description
			v.Description = &d
		}
	}
	if c.Attributes != nil {
		v.Attributes = c.Attributes
	}
	return v
}

// fieldsFor returns v's current values for the fields c changes: the
// "previous" half of the audit record.
func (v entryValues) fieldsFor(c EntryFields) EntryFields {
	var out EntryFields
	if c.Name != nil {
		n := v.Name
		out.Name = &n
	}
	if c.Description != nil {
		d := ""
		if v.Description != nil {
			d = *v.Description
		}
		out.Description = &d
	}
	if c.Attributes != nil {
		out.Attributes = v.Attributes
	}
	return out
}

// changedFields cleans the proposal and keeps only the fields that differ
// from cur. Attributes compare as JSON values, so key order and spacing
// don't count as changes.
func changedFields(cur entryValues, p EntryFields) (EntryFields, error) {
	var out EntryFields
	if p.Name != nil {
		if n := textnorm.Clean(*p.Name); n != cur.Name {
			out.Name = &n
		}
	}
	if p.Description != nil {
		d := textnorm.Clean(*p.Description)
		curD := ""
		if cur.Description != nil {
			curD = *cur.Description
		}
		if d != curD {
			out.Description = &d
		}
	}
	if p.Attributes != nil {
		attrs, err := cleanAttributes(p.Attributes)
		if err != nil {
			return EntryFields{}, err
		}
		same, err := jsonEqual(attrs, cur.Attributes)
		if err != nil {
			return EntryFields{}, err
		}
		if !same {
			out.Attributes = attrs
		}
	}
	return out, nil
}

// cleanAttributes trims and collapses whitespace in every top-level string
// value (textnorm.Clean), dropping values that end up empty. Numbers keep
// their exact JSON text.
func cleanAttributes(raw json.RawMessage) (json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("attributes must be a JSON object: %w", err)
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			if c := textnorm.Clean(s); c == "" {
				delete(m, k)
			} else {
				m[k] = c
			}
		}
	}
	return json.Marshal(m)
}

func jsonEqual(a, b json.RawMessage) (bool, error) {
	var x, y any
	for _, p := range []struct {
		raw json.RawMessage
		dst *any
	}{{a, &x}, {b, &y}} {
		dec := json.NewDecoder(bytes.NewReader(p.raw))
		dec.UseNumber()
		if err := dec.Decode(p.dst); err != nil {
			return false, err
		}
	}
	return reflect.DeepEqual(x, y), nil
}

func jsonOrNil(f *EntryFields) (*string, error) {
	if f == nil {
		return nil, nil
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	s := string(raw)
	return &s, nil
}

func cleanOptional(s *string) *string {
	if s == nil {
		return nil
	}
	c := textnorm.Clean(*s)
	if c == "" {
		return nil
	}
	return &c
}

// lockUser serializes one user's submissions so the pending cap can't be
// overrun by parallel requests. ErrNotFound if the user is gone. FOR NO KEY
// UPDATE doesn't block inserts elsewhere that reference the user (their
// FK checks take FOR KEY SHARE), only other submissions and deletion.
func lockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id = $1 FOR NO KEY UPDATE`, userID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func checkPendingCap(ctx context.Context, tx pgx.Tx, userID string) error {
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM moderation_requests WHERE submitted_by = $1 AND status = 'pending'`,
		userID).Scan(&n); err != nil {
		return fmt.Errorf("count pending: %w", err)
	}
	if n >= MaxPendingPerUser {
		return ErrSubmissionLimit
	}
	return nil
}
