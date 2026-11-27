package category

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// Limits on a user-built category (ADR 0013).
const (
	MaxFields     = 20
	MaxOptions    = 50
	MaxOptionLen  = 60
	maxSlugPrefix = 40
)

// Source is an approved category whose schema feeds the field library.
type Source struct {
	Slug   string
	Name   string
	Schema json.RawMessage
}

// FieldSource names a category a library field comes from, with the
// options that category offers for it (empty for free text). The builder
// starts a picked field with its source's options, so a field shared by many
// categories never arrives with everyone's options at once.
type FieldSource struct {
	Slug    string   `json:"slug"`
	Name    string   `json:"name"`
	Options []string `json:"options,omitempty"`
}

// LibraryField is a reusable attribute drawn from existing categories'
// schemas. Fields with the same normalized title and type are one field;
// their enum options are merged and every category using them is listed.
//
// ID is "<type>:<key>" and is what a builder request names. Key is the
// property name the field gets in a new schema: the folded title in
// snake_case, so it is the same in every category that uses the field.
type LibraryField struct {
	ID          string        `json:"id"`
	Key         string        `json:"key"`
	Title       string        `json:"title"`
	Type        string        `json:"type"`
	Description string        `json:"description,omitempty"`
	Minimum     *float64      `json:"minimum,omitempty"`
	Options     []string      `json:"options"`
	Sources     []FieldSource `json:"sources"`
}

// schemaProperty is the part of a JSON Schema property the builder reads
// and writes. The item form (AttributeFields) renders exactly these.
type schemaProperty struct {
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Minimum     *float64 `json:"minimum,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type objectSchema struct {
	Type                 string                    `json:"type"`
	Properties           map[string]schemaProperty `json:"properties"`
	AdditionalProperties bool                      `json:"additionalProperties"`
}

// renderable is the set of property types the item form can show.
var renderable = map[string]bool{"string": true, "integer": true, "number": true}

// FieldKey is the property name for a field title: folded to ASCII,
// lowercased, every run of other characters replaced by one underscore.
func FieldKey(title string) string {
	return joinRuns(textnorm.Fold(title), '_')
}

// FieldID identifies a library field: its type and key.
func FieldID(typ, title string) string {
	return typ + ":" + FieldKey(title)
}

// Library builds the field library from the given categories. Sources are
// read in category-name order, so the first category (by name) using a
// field decides its spelling and description.
func Library(sources []Source) []LibraryField {
	sorted := append([]Source(nil), sources...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return textnorm.Key(sorted[i].Name) < textnorm.Key(sorted[j].Name)
	})
	byID := map[string]*LibraryField{}
	for _, src := range sorted {
		for _, f := range fieldsOf(src.Schema) {
			fs := FieldSource{Slug: src.Slug, Name: src.Name}
			if len(f.Options) > 0 {
				fs.Options = append([]string(nil), f.Options...)
			}
			f.Sources = []FieldSource{fs}
			existing, ok := byID[f.ID]
			if !ok {
				f := f
				byID[f.ID] = &f
				continue
			}
			existing.Sources = append(existing.Sources, f.Sources...)
			existing.Options = NormalizeOptions(existing.Options, f.Options)
			if existing.Description == "" {
				existing.Description = f.Description
			}
			if existing.Minimum == nil {
				existing.Minimum = f.Minimum
			}
		}
	}
	out := make([]LibraryField, 0, len(byID))
	for _, f := range byID {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := textnorm.Key(out[i].Title), textnorm.Key(out[j].Title)
		if a != b {
			return a < b
		}
		return out[i].Type < out[j].Type
	})
	return out
}

// fieldsOf reads a schema's renderable properties as library fields, in
// property-name order. An unreadable schema yields none.
func fieldsOf(raw json.RawMessage) []LibraryField {
	var s struct {
		Properties map[string]schemaProperty `json:"properties"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &s) != nil {
		return nil
	}
	keys := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]LibraryField, 0, len(keys))
	for _, k := range keys {
		p := s.Properties[k]
		if !renderable[p.Type] {
			continue
		}
		title := textnorm.Clean(p.Title)
		if title == "" {
			title = humanize(k)
		}
		key := FieldKey(title)
		if key == "" {
			continue
		}
		out = append(out, LibraryField{
			ID:          p.Type + ":" + key,
			Key:         key,
			Title:       title,
			Type:        p.Type,
			Description: textnorm.Clean(p.Description),
			Minimum:     p.Minimum,
			Options:     NormalizeOptions(nil, p.Enum),
		})
	}
	return out
}

// NormalizeOptions appends given to base: each value is cleaned
// (textnorm.Clean), blanks are dropped, and a value matching one already
// in the list case-insensitively is not added again, so the existing
// spelling is reused. Casing is otherwise kept as typed ("LEGO", "UCS").
func NormalizeOptions(base, given []string) []string {
	out := make([]string, 0, len(base)+len(given))
	seen := map[string]bool{}
	for _, list := range [][]string{base, given} {
		for _, v := range list {
			c := textnorm.Clean(v)
			if c == "" || seen[textnorm.Key(c)] {
				continue
			}
			seen[textnorm.Key(c)] = true
			out = append(out, c)
		}
	}
	return out
}

// Choice is one field a user picked for their category. Options is the
// complete list they want (for text fields); empty means free text.
type Choice struct {
	ID      string   `json:"id"`
	Options []string `json:"options"`
}

// Composed is a generated category schema.
type Composed struct {
	Schema            json.RawMessage
	CatalogIdentifier *string
}

// ComposeError is a builder request the server refuses, with a message the
// user can act on.
type ComposeError struct {
	Message string
}

func (e *ComposeError) Error() string { return e.Message }

func composeErr(format string, args ...any) error {
	return &ComposeError{Message: fmt.Sprintf(format, args...)}
}

// Compose builds a category schema from the user's choices. A choice must
// name a library field or a field already in current (the category being
// edited), so edits keep fields even if no approved category uses them any
// more. Options are normalized against the field's existing options:
// typing "star wars" reuses "Star Wars". identifier, when set, is the ID
// of one of the chosen fields and becomes the catalog identifier.
func Compose(lib []LibraryField, current json.RawMessage, choices []Choice, identifier string) (Composed, error) {
	if len(choices) == 0 {
		return Composed{}, composeErr("Pick at least one field.")
	}
	if len(choices) > MaxFields {
		return Composed{}, composeErr("A category can have at most %d fields.", MaxFields)
	}
	available := map[string]LibraryField{}
	currentOptions := map[string][]string{}
	for _, f := range fieldsOf(current) {
		available[f.ID] = f
		currentOptions[f.ID] = f.Options
	}
	for _, f := range lib {
		available[f.ID] = f
	}

	props := map[string]schemaProperty{}
	var identifierKey *string
	for _, c := range choices {
		f, ok := available[c.ID]
		if !ok {
			return Composed{}, composeErr("One of the fields you picked isn't available. Reload and try again.")
		}
		if _, dup := props[f.Key]; dup {
			return Composed{}, composeErr("You picked two fields called %q. Keep one of them.", f.Title)
		}
		p := schemaProperty{Type: f.Type, Title: f.Title, Description: f.Description, Minimum: f.Minimum}
		if len(c.Options) > 0 {
			if f.Type != "string" {
				return Composed{}, composeErr("Only text fields can have a list of options (%q is a number).", f.Title)
			}
			for _, o := range c.Options {
				if utf8.RuneCountInString(textnorm.Clean(o)) > MaxOptionLen {
					return Composed{}, composeErr("Options must be at most %d characters.", MaxOptionLen)
				}
			}
			// Reuse a known spelling for options typed in another case (the
			// category's own current options first, so its items still
			// match, then the library's), but keep only the options listed.
			known := map[string]string{}
			for _, k := range NormalizeOptions(currentOptions[c.ID], f.Options) {
				known[textnorm.Key(k)] = k
			}
			opts := NormalizeOptions(nil, c.Options)
			for i, o := range opts {
				if k, ok := known[textnorm.Key(o)]; ok {
					opts[i] = k
				}
			}
			if len(opts) > MaxOptions {
				return Composed{}, composeErr("A field can have at most %d options.", MaxOptions)
			}
			if len(opts) > 0 {
				p.Enum = opts
			}
		}
		props[f.Key] = p
		if identifier != "" && identifier == c.ID {
			k := f.Key
			identifierKey = &k
		}
	}
	if identifier != "" && identifierKey == nil {
		return Composed{}, composeErr("The field that tells items apart must be one of the fields you picked.")
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(objectSchema{Type: "object", Properties: props, AdditionalProperties: false}); err != nil {
		return Composed{}, err
	}
	return Composed{Schema: json.RawMessage(bytes.TrimSpace(buf.Bytes())), CatalogIdentifier: identifierKey}, nil
}

// Slug is the URL slug for a user-built category: the folded name in
// kebab-case (at most 40 characters, "category" if nothing is left) plus a
// random suffix, so private categories with the same name never collide
// and a slug can't be guessed from a name.
func Slug(name, suffix string) string {
	base := joinRuns(textnorm.Fold(name), '-')
	if len(base) > maxSlugPrefix {
		base = strings.TrimRight(base[:maxSlugPrefix], "-")
	}
	if base == "" {
		base = "category"
	}
	return base + "-" + suffix
}

// joinRuns keeps ASCII letters and digits and replaces every run of other
// characters with sep, trimming sep from both ends.
func joinRuns(s string, sep byte) string {
	var b strings.Builder
	pending := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			if pending && b.Len() > 0 {
				b.WriteByte(sep)
			}
			pending = false
			b.WriteByte(c)
			continue
		}
		pending = true
	}
	return b.String()
}

func humanize(key string) string {
	s := strings.ReplaceAll(key, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
