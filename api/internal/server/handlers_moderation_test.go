package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// ---- fake -----------------------------------------------------------------

type submitCall struct {
	userID string
	in     store.SubmissionInput
}

type correctionCall struct {
	userID, entryID string
	proposed        store.EntryFields
	note            *string
}

type decisionCall struct {
	action, adminID, id string
	edits               *store.EntryFields
	reason              *string
	target              string
}

type fakeModeration struct {
	submittedCategory string
	submitResult      store.SubmitResult
	err               error

	submits     []submitCall
	corrections []correctionCall
	queueCalls  []store.QueueQuery
	decisions   []decisionCall

	request store.ModerationRequest
	mine    []store.Submission
}

func (f *fakeModeration) Submit(_ context.Context, userID string, in store.SubmissionInput) (store.SubmitResult, error) {
	f.submits = append(f.submits, submitCall{userID, in})
	if f.err != nil {
		return store.SubmitResult{}, f.err
	}
	return f.submitResult, nil
}

func (f *fakeModeration) ProposeCorrection(_ context.Context, userID, entryID string, p store.EntryFields, note *string) (store.Submission, error) {
	f.corrections = append(f.corrections, correctionCall{userID, entryID, p, note})
	if f.err != nil {
		return store.Submission{}, f.err
	}
	return store.Submission{ID: "req-1", Kind: store.KindCatalogCorrection, Status: store.StatusPending, Proposed: &p}, nil
}

func (f *fakeModeration) ListMine(_ context.Context, _ string) ([]store.Submission, error) {
	return f.mine, nil
}

func (f *fakeModeration) Queue(_ context.Context, q store.QueueQuery) (store.QueuePage, error) {
	f.queueCalls = append(f.queueCalls, q)
	return store.QueuePage{Requests: []store.QueueItem{}}, nil
}

func (f *fakeModeration) Get(_ context.Context, id string) (store.ModerationRequest, error) {
	if id != f.request.ID {
		return store.ModerationRequest{}, store.ErrNotFound
	}
	return f.request, nil
}

func (f *fakeModeration) Approve(_ context.Context, adminID, id string, edits *store.EntryFields) (store.ModerationRequest, error) {
	f.decisions = append(f.decisions, decisionCall{action: "approve", adminID: adminID, id: id, edits: edits})
	if f.err != nil {
		return store.ModerationRequest{}, f.err
	}
	return f.request, nil
}

func (f *fakeModeration) Reject(_ context.Context, adminID, id, reason string) (store.ModerationRequest, error) {
	f.decisions = append(f.decisions, decisionCall{action: "reject", adminID: adminID, id: id, reason: &reason})
	if f.err != nil {
		return store.ModerationRequest{}, f.err
	}
	return f.request, nil
}

func (f *fakeModeration) SubmitCategory(_ context.Context, userID, slug string, note *string) (store.Submission, error) {
	f.submittedCategory = slug
	if f.err != nil {
		return store.Submission{}, f.err
	}
	return store.Submission{ID: "req-cat", Kind: store.KindCategory, Status: store.StatusPending, CategorySlug: slug}, nil
}

func (f *fakeModeration) Merge(_ context.Context, adminID, id, target string, reason *string) (store.ModerationRequest, error) {
	f.decisions = append(f.decisions, decisionCall{action: "merge", adminID: adminID, id: id, target: target, reason: reason})
	if f.err != nil {
		return store.ModerationRequest{}, f.err
	}
	return f.request, nil
}

// ---- helpers ----------------------------------------------------------------

const testRequestID = "22222222-2222-4222-8222-222222222222"

func legoSchemaCategories() *fakeCategoriesWithSchema {
	return &fakeCategoriesWithSchema{schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"set_number": {"type": "string"},
			"piece_count": {"type": "integer", "minimum": 0}
		},
		"additionalProperties": false
	}`)}
}

// newModerationRouter signs every request in as user-1, whose role is
// `role` ("" for a regular user).
func newModerationRouter(t *testing.T, mod *fakeModeration, role string) http.Handler {
	t.Helper()
	user := store.User{ID: "user-1", Email: "u@x", DisplayName: "U"}
	if role != "" {
		user.Role = &role
	}
	return newAuthenticatedRouterWithDeps(t, server.Deps{
		Categories: legoSchemaCategories(),
		Catalog:    &fakeCatalog{entries: []store.CatalogEntry{falconEntry()}},
		Moderation: mod,
		Users:      &fakeUsers{user: user},
	}, "user-1")
}

func pendingEntryRequest() store.ModerationRequest {
	e := falconEntry()
	e.Status = "pending"
	return store.ModerationRequest{
		ID: testRequestID, Kind: store.KindCatalogEntry, Status: store.StatusPending,
		CategorySlug: "lego-sets", CategoryName: "Lego Sets", Entry: &e,
	}
}

// ---- role gate --------------------------------------------------------------

var adminRoutes = []struct{ method, path string }{
	{http.MethodGet, "/admin/moderation"},
	{http.MethodGet, "/admin/moderation/" + testRequestID},
	{http.MethodPost, "/admin/moderation/" + testRequestID + "/approve"},
	{http.MethodPost, "/admin/moderation/" + testRequestID + "/reject"},
	{http.MethodPost, "/admin/moderation/" + testRequestID + "/merge"},
}

func TestAdminRoutesRequireSignIn(t *testing.T) {
	r := server.NewRouter(server.Deps{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		DBPinger:   fakePinger{},
		Categories: &fakeCategories{},
		Moderation: &fakeModeration{},
		Users:      &fakeUsers{},
		Sessions:   auth.NewManager([]byte("01234567890123456789012345678901"), false),
	})
	for _, tc := range adminRoutes {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, http.StatusUnauthorized, rr.Code, tc.method+" "+tc.path)
	}
}

func TestAdminRoutesForbidRegularUsers(t *testing.T) {
	mod := &fakeModeration{request: pendingEntryRequest()}
	r := newModerationRouter(t, mod, "")
	for _, tc := range adminRoutes {
		rr := doJSON(t, r, tc.method, tc.path, map[string]any{"reason": "x", "catalog_entry_id": "x"})
		require.Equal(t, http.StatusForbidden, rr.Code, tc.method+" "+tc.path)
	}
	require.Empty(t, mod.decisions, "the store is never reached")
	require.Empty(t, mod.queueCalls)
}

func TestAdminRoutesAllowAdmins(t *testing.T) {
	mod := &fakeModeration{request: pendingEntryRequest()}
	r := newModerationRouter(t, mod, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodGet, "/admin/moderation", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	rr = doJSON(t, r, http.MethodGet, "/admin/moderation/"+testRequestID, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestMeIncludesRole(t *testing.T) {
	r := newModerationRouter(t, &fakeModeration{}, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodGet, "/me", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `"admin"`, string(mustField(t, rr.Body.Bytes(), "user", "role")))

	r = newModerationRouter(t, &fakeModeration{}, "")
	rr = doJSON(t, r, http.MethodGet, "/me", nil)
	require.JSONEq(t, `null`, string(mustField(t, rr.Body.Bytes(), "user", "role")))
}

func mustField(t *testing.T, body []byte, path ...string) json.RawMessage {
	t.Helper()
	raw := json.RawMessage(body)
	for _, key := range path {
		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &m))
		v, ok := m[key]
		require.True(t, ok, "missing %q in %s", key, raw)
		raw = v
	}
	return raw
}

// ---- submissions ------------------------------------------------------------

func TestSubmitCatalogEntry(t *testing.T) {
	mod := &fakeModeration{submitResult: store.SubmitResult{
		Outcome: store.OutcomeSubmitted, Submission: &store.Submission{ID: "req-1", Status: store.StatusPending},
	}}
	r := newModerationRouter(t, mod, "")
	item := "33333333-3333-4333-8333-333333333333"
	rr := doJSON(t, r, http.MethodPost, "/me/catalog-submissions", map[string]any{
		"category_slug": "lego-sets", "name": "  Ultimate   Tower ",
		"attributes": map[string]any{"set_number": "10305"}, "item_id": item,
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.Len(t, mod.submits, 1)
	require.Equal(t, "user-1", mod.submits[0].userID)
	require.Equal(t, "Ultimate Tower", mod.submits[0].in.Name)
	require.Equal(t, "lego-sets", mod.submits[0].in.CategorySlug)
	require.Equal(t, item, *mod.submits[0].in.SourceItemID)
	require.JSONEq(t, `"submitted"`, string(mustField(t, rr.Body.Bytes(), "outcome")))
}

func TestSubmitCatalogEntryOmittedAttributesAreValidatedAsEmpty(t *testing.T) {
	mod := &fakeModeration{submitResult: store.SubmitResult{Outcome: store.OutcomeSubmitted}}
	r := newModerationRouter(t, mod, "")
	rr := doJSON(t, r, http.MethodPost, "/me/catalog-submissions", map[string]any{"category_slug": "lego-sets", "name": "X"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.JSONEq(t, `{}`, string(mod.submits[0].in.Attributes))
}

func TestSubmitCatalogEntryExistingMatchIs200(t *testing.T) {
	e := falconEntry()
	mod := &fakeModeration{submitResult: store.SubmitResult{Outcome: store.OutcomeAlreadyInCatalog, Entry: &e}}
	r := newModerationRouter(t, mod, "")
	rr := doJSON(t, r, http.MethodPost, "/me/catalog-submissions", map[string]any{
		"category_slug": "lego-sets", "name": "Millennium Falcon", "attributes": map[string]any{"set_number": "75192"},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestSubmitCatalogEntryValidation(t *testing.T) {
	mod := &fakeModeration{}
	r := newModerationRouter(t, mod, "")
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	cases := map[string]map[string]any{
		"blank name":      {"category_slug": "lego-sets", "name": "   "},
		"long name":       {"category_slug": "lego-sets", "name": string(long)},
		"bad slug":        {"category_slug": "Not A Slug", "name": "X"},
		"schema mismatch": {"category_slug": "lego-sets", "name": "X", "attributes": map[string]any{"piece_count": "lots"}},
		"unknown field":   {"category_slug": "lego-sets", "name": "X", "status": "approved"},
	}
	for name, body := range cases {
		rr := doJSON(t, r, http.MethodPost, "/me/catalog-submissions", body)
		require.Equal(t, http.StatusBadRequest, rr.Code, name)
	}
	rr := doJSON(t, r, http.MethodPost, "/me/catalog-submissions", map[string]any{"category_slug": "coins", "name": "X"})
	require.Equal(t, http.StatusNotFound, rr.Code, "unknown category")
	require.Empty(t, mod.submits)
}

func TestSubmitCatalogEntryErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{store.ErrNotFound, http.StatusNotFound},
		{store.ErrSubmissionLimit, http.StatusTooManyRequests},
	} {
		r := newModerationRouter(t, &fakeModeration{err: tc.err}, "")
		rr := doJSON(t, r, http.MethodPost, "/me/catalog-submissions", map[string]any{"category_slug": "lego-sets", "name": "X"})
		require.Equal(t, tc.want, rr.Code, tc.err.Error())
	}
}

func TestListMySubmissions(t *testing.T) {
	mod := &fakeModeration{mine: []store.Submission{{ID: "req-1", Name: "Tower", Status: store.StatusRejected}}}
	r := newModerationRouter(t, mod, "")
	rr := doJSON(t, r, http.MethodGet, "/me/catalog-submissions", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var payload struct {
		Submissions []store.Submission `json:"submissions"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	require.Len(t, payload.Submissions, 1)
}

// ---- corrections ------------------------------------------------------------

func TestProposeCorrection(t *testing.T) {
	mod := &fakeModeration{}
	r := newModerationRouter(t, mod, "")
	rr := doJSON(t, r, http.MethodPost, "/me/catalog-corrections", map[string]any{
		"catalog_entry_id": falconEntry().ID, "name": "Millennium Falcon UCS",
		"attributes": map[string]any{"set_number": "75192", "piece_count": 7541}, "note": " From the box ",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.Len(t, mod.corrections, 1)
	c := mod.corrections[0]
	require.Equal(t, falconEntry().ID, c.entryID)
	require.Equal(t, "Millennium Falcon UCS", *c.proposed.Name)
	require.Equal(t, "From the box", *c.note)
}

func TestProposeCorrectionValidation(t *testing.T) {
	mod := &fakeModeration{}
	r := newModerationRouter(t, mod, "")
	for name, body := range map[string]map[string]any{
		"nothing proposed": {"catalog_entry_id": falconEntry().ID},
		"blank name":       {"catalog_entry_id": falconEntry().ID, "name": " "},
		"schema mismatch":  {"catalog_entry_id": falconEntry().ID, "attributes": map[string]any{"piece_count": -1}},
		"missing entry id": {"name": "X"},
	} {
		rr := doJSON(t, r, http.MethodPost, "/me/catalog-corrections", body)
		require.Equal(t, http.StatusBadRequest, rr.Code, name)
	}
	rr := doJSON(t, r, http.MethodPost, "/me/catalog-corrections", map[string]any{
		"catalog_entry_id": "44444444-4444-4444-8444-444444444444", "name": "X",
	})
	require.Equal(t, http.StatusNotFound, rr.Code, "unknown or unapproved entry")
	require.Empty(t, mod.corrections)

	r = newModerationRouter(t, &fakeModeration{err: store.ErrNoChanges}, "")
	rr = doJSON(t, r, http.MethodPost, "/me/catalog-corrections", map[string]any{"catalog_entry_id": falconEntry().ID, "name": "Millennium Falcon"})
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// ---- admin ------------------------------------------------------------------

func TestModerationQueueParams(t *testing.T) {
	mod := &fakeModeration{}
	r := newModerationRouter(t, mod, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodGet, "/admin/moderation?kind=catalog_correction&category_slug=lego-sets&limit=10&offset=20", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, store.QueueQuery{Kind: store.KindCatalogCorrection, CategorySlug: "lego-sets", Limit: 10, Offset: 20}, mod.queueCalls[0])

	rr = doJSON(t, r, http.MethodGet, "/admin/moderation", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, store.QueueQuery{Limit: 25}, mod.queueCalls[1])

	for _, q := range []string{"kind=everything", "limit=0", "limit=101", "offset=-1", "category_slug=Bad%20Slug"} {
		rr := doJSON(t, r, http.MethodGet, "/admin/moderation?"+q, nil)
		require.Equal(t, http.StatusBadRequest, rr.Code, q)
	}
}

func TestModerationGetUnknown404(t *testing.T) {
	r := newModerationRouter(t, &fakeModeration{request: pendingEntryRequest()}, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodGet, "/admin/moderation/55555555-5555-4555-8555-555555555555", nil)
	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestModerationApprove(t *testing.T) {
	mod := &fakeModeration{request: pendingEntryRequest()}
	r := newModerationRouter(t, mod, auth.RoleAdmin)

	rr := doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/approve", map[string]any{})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "user-1", mod.decisions[0].adminID)
	require.Nil(t, mod.decisions[0].edits, "no edits means approve as submitted")

	rr = doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/approve", map[string]any{
		"name": " Hogwarts  Castle ", "attributes": map[string]any{"set_number": "71043"},
	})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "Hogwarts Castle", *mod.decisions[1].edits.Name)

	rr = doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/approve", map[string]any{
		"attributes": map[string]any{"piece_count": "many"},
	})
	require.Equal(t, http.StatusBadRequest, rr.Code, "edits are validated against the schema")
	require.Len(t, mod.decisions, 2)
}

func TestModerationRejectNeedsReason(t *testing.T) {
	mod := &fakeModeration{request: pendingEntryRequest()}
	r := newModerationRouter(t, mod, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/reject", map[string]any{"reason": "  "})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	rr = doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/reject", map[string]any{"reason": " Needs a set number. "})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "Needs a set number.", *mod.decisions[0].reason)
}

func TestModerationMerge(t *testing.T) {
	mod := &fakeModeration{request: pendingEntryRequest()}
	r := newModerationRouter(t, mod, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/merge", map[string]any{})
	require.Equal(t, http.StatusBadRequest, rr.Code, "a target is required")
	rr = doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/merge", map[string]any{"catalog_entry_id": falconEntry().ID})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, falconEntry().ID, mod.decisions[0].target)
}

func TestModerationDecisionErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{store.ErrAlreadyDecided, http.StatusConflict},
		{store.ErrDuplicateEntry, http.StatusConflict},
		{store.ErrMergeTarget, http.StatusBadRequest},
		{store.ErrWrongKind, http.StatusBadRequest},
		{store.ErrEntryNotPending, http.StatusConflict},
		{store.ErrNotFound, http.StatusNotFound},
	} {
		r := newModerationRouter(t, &fakeModeration{request: pendingEntryRequest(), err: tc.err}, auth.RoleAdmin)
		rr := doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/merge", map[string]any{"catalog_entry_id": falconEntry().ID})
		require.Equal(t, tc.want, rr.Code, tc.err.Error())
	}
}
