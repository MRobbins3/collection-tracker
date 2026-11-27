package category_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
)

var librarySources = []category.Source{
	{Slug: "vinyl-records", Name: "Vinyl Records", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"artist": {"type": "string", "title": "Artist"},
			"year": {"type": "integer", "title": "year"},
			"pressing": {"type": "string", "title": "Pressing", "enum": ["Original", "Reissue"]}
		},
		"additionalProperties": false
	}`)},
	{Slug: "lego-sets", Name: "Lego Sets", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"set_number": {"type": "string", "title": "Set number", "description": "The number on the box."},
			"theme": {"type": "string", "title": "Theme", "enum": ["star wars", "Technic"]},
			"piece_count": {"type": "integer", "title": "Piece count", "minimum": 0}
		},
		"additionalProperties": false
	}`)},
	{Slug: "coins", Name: "Coins", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"year": {"type": "integer", "title": "Year"},
			"theme": {"type": "string", "title": "Theme", "enum": ["Star Wars", "Presidents"]},
			"weird": {"type": "boolean", "title": "Weird"}
		}
	}`)},
	{Slug: "stamps", Name: "Stamps", Schema: json.RawMessage(`{
		"type": "object",
		"properties": {
			"year": {"type": "string", "title": "Year"}
		}
	}`)},
}

func findField(t *testing.T, lib []category.LibraryField, id string) category.LibraryField {
	t.Helper()
	for _, f := range lib {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("field %q not in library", id)
	return category.LibraryField{}
}

func TestLibrary(t *testing.T) {
	lib := category.Library(librarySources)

	t.Run("dedupes by normalized title and type, listing every source", func(t *testing.T) {
		year := findField(t, lib, "integer:year")
		require.Equal(t, "Year", year.Title, "the first source by category name wins the spelling")
		require.Equal(t, "year", year.Key)
		require.Equal(t, []category.FieldSource{
			{Slug: "coins", Name: "Coins"}, {Slug: "vinyl-records", Name: "Vinyl Records"},
		}, year.Sources)
		// Same title, different type: a separate field.
		yearText := findField(t, lib, "string:year")
		require.Equal(t, []category.FieldSource{{Slug: "stamps", Name: "Stamps"}}, yearText.Sources)
	})

	t.Run("merges enum options case-insensitively, keeping the first spelling", func(t *testing.T) {
		theme := findField(t, lib, "string:theme")
		require.Equal(t, []string{"Star Wars", "Presidents", "Technic"}, theme.Options)
		require.Equal(t, []category.FieldSource{
			{Slug: "coins", Name: "Coins", Options: []string{"Star Wars", "Presidents"}},
			{Slug: "lego-sets", Name: "Lego Sets", Options: []string{"star wars", "Technic"}},
		}, theme.Sources, "each source keeps its own options")
	})

	t.Run("keeps descriptions and minimums", func(t *testing.T) {
		require.Equal(t, "The number on the box.", findField(t, lib, "string:set_number").Description)
		pc := findField(t, lib, "integer:piece_count")
		require.NotNil(t, pc.Minimum)
		require.Equal(t, float64(0), *pc.Minimum)
		require.Empty(t, findField(t, lib, "string:artist").Options)
	})

	t.Run("skips types the item form can't render", func(t *testing.T) {
		for _, f := range lib {
			require.NotEqual(t, "boolean", f.Type)
		}
	})

	t.Run("is ordered by title", func(t *testing.T) {
		var titles []string
		for _, f := range lib {
			titles = append(titles, f.Title+"/"+f.Type)
		}
		require.Equal(t, []string{
			"Artist/string", "Piece count/integer", "Pressing/string", "Set number/string",
			"Theme/string", "Year/integer", "Year/string",
		}, titles)
	})
}

func TestNormalizeOptions(t *testing.T) {
	got := category.NormalizeOptions(
		[]string{"Star Wars", "Technic"},
		[]string{"  star   wars ", "LEGO Ideas", "lego ideas", "", "Technic", "UCS"},
	)
	require.Equal(t, []string{"Star Wars", "Technic", "LEGO Ideas", "UCS"}, got,
		"trimmed and collapsed; existing spellings reused case-insensitively; duplicates and blanks dropped; deliberate casing kept")
}

func TestCompose(t *testing.T) {
	lib := category.Library(librarySources)

	t.Run("builds a schema the validator accepts", func(t *testing.T) {
		got, err := category.Compose(lib, nil, []category.Choice{
			{ID: "string:set_number"},
			{ID: "string:theme", Options: []string{"star wars", "Ninjago"}},
			{ID: "string:pressing", Options: []string{"Original", "reissue", "Picture Disc"}},
			{ID: "integer:piece_count"},
		}, "string:set_number")
		require.NoError(t, err)
		require.NotNil(t, got.CatalogIdentifier)
		require.Equal(t, "set_number", *got.CatalogIdentifier)

		var schema struct {
			Type                 string                    `json:"type"`
			AdditionalProperties bool                      `json:"additionalProperties"`
			Properties           map[string]map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(got.Schema, &schema))
		require.Equal(t, "object", schema.Type)
		require.False(t, schema.AdditionalProperties)
		require.Len(t, schema.Properties, 4)
		require.Equal(t, "Theme", schema.Properties["theme"]["title"])
		require.Equal(t, []any{"Star Wars", "Ninjago"}, schema.Properties["theme"]["enum"])
		require.Equal(t, []any{"Original", "Reissue", "Picture Disc"}, schema.Properties["pressing"]["enum"])
		require.Equal(t, "The number on the box.", schema.Properties["set_number"]["description"])
		require.Equal(t, float64(0), schema.Properties["piece_count"]["minimum"])

		require.NoError(t, category.ValidateAttributes(got.Schema,
			json.RawMessage(`{"set_number":"75192","theme":"Ninjago","pressing":"Picture Disc","piece_count":10}`)))
		var ve *category.ValidationError
		require.True(t, errors.As(category.ValidateAttributes(got.Schema, json.RawMessage(`{"theme":"Space"}`)), &ve),
			"a value outside the options is refused")
		require.True(t, errors.As(category.ValidateAttributes(got.Schema, json.RawMessage(`{"color":"red"}`)), &ve),
			"unknown attributes are refused")
	})

	t.Run("a field with no options stays free text", func(t *testing.T) {
		got, err := category.Compose(lib, nil, []category.Choice{{ID: "string:artist"}}, "")
		require.NoError(t, err)
		require.Nil(t, got.CatalogIdentifier)
		require.NoError(t, category.ValidateAttributes(got.Schema, json.RawMessage(`{"artist":"Anyone"}`)))
	})

	t.Run("options removed from a library field are dropped", func(t *testing.T) {
		got, err := category.Compose(lib, nil, []category.Choice{{ID: "string:theme", Options: []string{"Technic"}}}, "")
		require.NoError(t, err)
		require.NoError(t, category.ValidateAttributes(got.Schema, json.RawMessage(`{"theme":"Technic"}`)))
		require.Error(t, category.ValidateAttributes(got.Schema, json.RawMessage(`{"theme":"Star Wars"}`)))
	})

	t.Run("fields already in the category stay choosable", func(t *testing.T) {
		current := json.RawMessage(`{"type":"object","properties":{"color":{"type":"string","title":"Color","enum":["Red"]}}}`)
		got, err := category.Compose(lib, current, []category.Choice{{ID: "string:color", Options: []string{"red", "Blue"}}}, "")
		require.NoError(t, err)
		require.Contains(t, string(got.Schema), `"enum":["Red","Blue"]`)
	})

	t.Run("an edit reuses the category's own option spellings before the library's", func(t *testing.T) {
		current := json.RawMessage(`{"type":"object","properties":{"theme":{"type":"string","title":"Theme","enum":["Clone Wars","STAR WARS"]}}}`)
		got, err := category.Compose(lib, current, []category.Choice{
			{ID: "string:theme", Options: []string{"clone wars", "Clone Wars", "star wars"}},
		}, "")
		require.NoError(t, err)
		require.Contains(t, string(got.Schema), `"enum":["Clone Wars","STAR WARS"]`,
			"no duplicate, and the spelling the items already use wins over the library's \"Star Wars\"")
	})

	refusals := []struct {
		name       string
		choices    []category.Choice
		identifier string
	}{
		{"no fields", nil, ""},
		{"unknown field", []category.Choice{{ID: "string:secret"}}, ""},
		{"the same field twice", []category.Choice{{ID: "string:artist"}, {ID: "string:artist"}}, ""},
		{"two fields with the same key", []category.Choice{{ID: "string:year"}, {ID: "integer:year"}}, ""},
		{"options on a number field", []category.Choice{{ID: "integer:year", Options: []string{"1999"}}}, ""},
		{"an option that is too long", []category.Choice{{ID: "string:theme", Options: []string{string(make([]byte, 61))}}}, ""},
		{"an identifier that isn't chosen", []category.Choice{{ID: "string:artist"}}, "string:set_number"},
	}
	for _, tc := range refusals {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			_, err := category.Compose(lib, nil, tc.choices, tc.identifier)
			var ce *category.ComposeError
			require.True(t, errors.As(err, &ce), "got %v", err)
			require.NotEmpty(t, ce.Error())
		})
	}

	t.Run("refuses too many fields or options", func(t *testing.T) {
		many := make([]category.Choice, category.MaxFields+1)
		for i := range many {
			many[i] = category.Choice{ID: "string:artist"}
		}
		_, err := category.Compose(lib, nil, many, "")
		require.Error(t, err)
		opts := make([]string, category.MaxOptions+1)
		for i := range opts {
			opts[i] = "Option " + string(rune('A'+i%26)) + string(rune('a'+i/26))
		}
		_, err = category.Compose(lib, nil, []category.Choice{{ID: "string:theme", Options: opts}}, "")
		require.Error(t, err)
	})
}

func TestSlug(t *testing.T) {
	require.Equal(t, "sneakers-ab12cd", category.Slug("  Sneakers ", "ab12cd"))
	require.Equal(t, "pokemon-cards-ab12cd", category.Slug("Pokémon Cards!", "ab12cd"))
	require.Equal(t, "category-ab12cd", category.Slug("日本", "ab12cd"))
	long := category.Slug("a very long category name that goes on and on and on forever", "ab12cd")
	require.LessOrEqual(t, len(long), 48)
	require.Regexp(t, `^[a-z0-9]+(-[a-z0-9]+)*$`, long)
}
