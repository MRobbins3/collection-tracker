package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func builderSources() []category.Source {
	return []category.Source{
		{Slug: "lego-sets", Name: "Lego Sets", Schema: json.RawMessage(`{"type":"object","properties":{
			"set_number":{"type":"string","title":"Set number"},
			"theme":{"type":"string","title":"Theme"}}}`)},
		{Slug: "vinyl-records", Name: "Vinyl Records", Schema: json.RawMessage(`{"type":"object","properties":{
			"year":{"type":"integer","title":"Year"}}}`)},
	}
}

func newCategoriesRouter(t *testing.T, cats *fakeCategories, mod *fakeModeration) http.Handler {
	t.Helper()
	if mod == nil {
		mod = &fakeModeration{}
	}
	return newAuthenticatedRouterWithDeps(t, server.Deps{
		Categories: cats,
		Moderation: mod,
		Users:      &fakeUsers{user: store.User{ID: "user-1", Email: "u@x", DisplayName: "U"}},
	}, "user-1")
}

func TestCategoryReadsPassTheViewer(t *testing.T) {
	cats := &fakeCategories{list: []store.Category{{ID: "c1", Slug: "lego-sets", Name: "Lego Sets"}}}
	r := newCategoriesRouter(t, cats, nil)
	for _, path := range []string{"/categories", "/categories?q=lego", "/categories/lego-sets"} {
		rr := doJSON(t, r, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, rr.Code, path)
	}
	require.Equal(t, []string{"user-1", "user-1", "user-1"}, cats.viewers)

	anon := &fakeCategories{}
	rr := doJSON(t, newRouterWith(anon, fakePinger{}), http.MethodGet, "/categories", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, []string{""}, anon.viewers, "anonymous callers read as nobody")
}

func TestListCategoryFields(t *testing.T) {
	cats := &fakeCategories{sources: builderSources()}
	rr := doJSON(t, newRouterWith(cats, fakePinger{}), http.MethodGet, "/category-fields", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body struct {
		Fields []category.LibraryField `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Fields, 3)
	require.Equal(t, "string:set_number", body.Fields[0].ID)
	require.Equal(t, "Lego Sets", body.Fields[0].Sources[0].Name)
}

func TestCreateMyCategory(t *testing.T) {
	cats := &fakeCategories{sources: builderSources()}
	r := newCategoriesRouter(t, cats, nil)
	rr := doJSON(t, r, http.MethodPost, "/me/categories", map[string]any{
		"name":        "  Sneakers ",
		"description": " Shoes   I collect ",
		"fields": []map[string]any{
			{"id": "string:theme", "options": []string{" nike ", "Nike", "Adidas"}},
			{"id": "integer:year"},
		},
		"catalog_identifier": "string:theme",
	})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.NotNil(t, cats.created)
	require.Equal(t, "Sneakers", cats.created.Name)
	require.Equal(t, "Shoes I collect", *cats.created.Description)
	require.Equal(t, "theme", *cats.created.CatalogIdentifier)
	require.Contains(t, string(cats.created.Schema), `"enum":["nike","Adidas"]`)
	require.NoError(t, category.ValidateAttributes(cats.created.Schema, json.RawMessage(`{"theme":"Adidas","year":2020}`)))
}

func TestCreateMyCategoryRefusals(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		status int
		want   string
	}{
		{"blank name", map[string]any{"name": "  ", "fields": []map[string]any{{"id": "integer:year"}}}, 400, "name"},
		{"long name", map[string]any{"name": strings.Repeat("x", 61), "fields": []map[string]any{{"id": "integer:year"}}}, 400, "name"},
		{"no fields", map[string]any{"name": "Shoes", "fields": []map[string]any{}}, 400, "at least one field"},
		{"unknown field", map[string]any{"name": "Shoes", "fields": []map[string]any{{"id": "string:secret"}}}, 400, "isn't available"},
		{"custom field shape", map[string]any{"name": "Shoes", "fields": []map[string]any{{"id": "string:theme", "type": "boolean"}}}, 400, "invalid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cats := &fakeCategories{sources: builderSources()}
			rr := doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPost, "/me/categories", tc.body)
			require.Equal(t, tc.status, rr.Code, rr.Body.String())
			require.Contains(t, rr.Body.String(), tc.want)
			require.Nil(t, cats.created)
		})
	}

	t.Run("a name you can already use is a 409 naming that category", func(t *testing.T) {
		cats := &fakeCategories{sources: builderSources(), createErr: &store.CategoryExistsError{
			Existing: store.Category{Slug: "lego-sets", Name: "Lego Sets"},
		}}
		rr := doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPost, "/me/categories", map[string]any{
			"name": "lego sets", "fields": []map[string]any{{"id": "integer:year"}},
		})
		require.Equal(t, http.StatusConflict, rr.Code)
		require.JSONEq(t, `{"slug":"lego-sets","name":"Lego Sets"}`, string(mustField(t, rr.Body.Bytes(), "existing")))
	})
}

func TestUpdateMyCategory(t *testing.T) {
	mine := store.Category{ID: "c1", Slug: "shoes-abc123", Name: "Shoes", Status: store.CategoryPrivate, Mine: true,
		AttributeSchema: json.RawMessage(`{"type":"object","properties":{"color":{"type":"string","title":"Color"}}}`)}
	others := store.Category{ID: "c2", Slug: "lego-sets", Name: "Lego Sets", Status: store.CategoryApproved}
	pending := mine
	pending.Slug, pending.Status = "pending-abc123", store.CategoryPending

	t.Run("edits your own, keeping fields only it has", func(t *testing.T) {
		cats := &fakeCategories{sources: builderSources(), list: []store.Category{mine}}
		rr := doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPatch, "/me/categories/shoes-abc123", map[string]any{
			"name": "Shoes", "fields": []map[string]any{{"id": "string:color", "options": []string{"Red"}}, {"id": "integer:year"}},
		})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Contains(t, string(cats.updated.Schema), `"color"`)
	})

	t.Run("someone else's (or a system) category is 404", func(t *testing.T) {
		cats := &fakeCategories{sources: builderSources(), list: []store.Category{others}}
		rr := doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPatch, "/me/categories/lego-sets", map[string]any{
			"name": "Lego", "fields": []map[string]any{{"id": "integer:year"}},
		})
		require.Equal(t, http.StatusNotFound, rr.Code)
		rr = doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPatch, "/me/categories/nope", map[string]any{
			"name": "Lego", "fields": []map[string]any{{"id": "integer:year"}},
		})
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("a submitted category is locked", func(t *testing.T) {
		cats := &fakeCategories{sources: builderSources(), list: []store.Category{pending}}
		rr := doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPatch, "/me/categories/pending-abc123", map[string]any{
			"name": "Shoes", "fields": []map[string]any{{"id": "integer:year"}},
		})
		require.Equal(t, http.StatusConflict, rr.Code)
		require.Nil(t, cats.updated)
	})

	t.Run("an edit that would orphan item values is a 409 with the reason", func(t *testing.T) {
		cats := &fakeCategories{sources: builderSources(), list: []store.Category{mine},
			updateErr: &store.SchemaInUseError{Message: "1 item has a value for \"Color\"."}}
		rr := doJSON(t, newCategoriesRouter(t, cats, nil), http.MethodPatch, "/me/categories/shoes-abc123", map[string]any{
			"name": "Shoes", "fields": []map[string]any{{"id": "integer:year"}},
		})
		require.Equal(t, http.StatusConflict, rr.Code)
		require.Contains(t, rr.Body.String(), "Color")
	})
}

func TestSubmitMyCategory(t *testing.T) {
	mod := &fakeModeration{}
	rr := doJSON(t, newCategoriesRouter(t, &fakeCategories{}, mod), http.MethodPost, "/me/categories/shoes-abc123/submit",
		map[string]any{"note": "Please"})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	require.Equal(t, "shoes-abc123", mod.submittedCategory)

	for err, status := range map[error]int{
		store.ErrNotFound:        http.StatusNotFound,
		store.ErrCategoryLocked:  http.StatusConflict,
		store.ErrSubmissionLimit: http.StatusTooManyRequests,
	} {
		rr := doJSON(t, newCategoriesRouter(t, &fakeCategories{}, &fakeModeration{err: err}), http.MethodPost,
			"/me/categories/shoes-abc123/submit", map[string]any{})
		require.Equal(t, status, rr.Code, err.Error())
	}
}

func TestCatalogSuggestionsNeedAPublicCategory(t *testing.T) {
	cats := &fakeCategories{list: []store.Category{{ID: "c1", Slug: "shoes-abc123", Name: "Shoes",
		Status: store.CategoryPrivate, Mine: true, AttributeSchema: json.RawMessage(`{}`)}}}
	mod := &fakeModeration{}
	rr := doJSON(t, newCategoriesRouter(t, cats, mod), http.MethodPost, "/me/catalog-submissions", map[string]any{
		"category_slug": "shoes-abc123", "name": "Air Max",
	})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "public list")
	require.Empty(t, mod.submits)
}

func TestCategoryRequestsAreApprovedAsBuilt(t *testing.T) {
	req := store.ModerationRequest{ID: testRequestID, Kind: store.KindCategory, Status: store.StatusPending,
		CategorySlug: "shoes-abc123", CategoryName: "Shoes"}
	mod := &fakeModeration{request: req}
	r := newModerationRouter(t, mod, auth.RoleAdmin)
	rr := doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/approve", map[string]any{"name": "Shoes!"})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "as their creator built them")

	rr = doJSON(t, r, http.MethodPost, "/admin/moderation/"+testRequestID+"/approve", map[string]any{})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	rr = doJSON(t, newModerationRouter(t, &fakeModeration{request: req, err: store.ErrDuplicateCategory}, auth.RoleAdmin),
		http.MethodPost, "/admin/moderation/"+testRequestID+"/approve", map[string]any{})
	require.Equal(t, http.StatusConflict, rr.Code)
	require.Contains(t, rr.Body.String(), "already on the public list")

	rr = doJSON(t, r, http.MethodGet, "/admin/moderation?kind=category", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}
