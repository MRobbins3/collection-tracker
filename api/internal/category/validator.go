// Package category validates user-submitted item attributes against the
// category's JSON Schema. Keeps the schema-validation concern out of store
// and handler code so both can call it without pulling gojsonschema into
// their own imports.
package category

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// ValidationError carries a human-friendly summary plus the per-field
// messages from gojsonschema so the frontend can surface them individually.
type ValidationError struct {
	Summary string
	Fields  map[string]string
}

func (e *ValidationError) Error() string { return e.Summary }

// ErrEmptySchema is returned when the category has no attribute_schema, which
// we treat as "allow anything" rather than an error — but callers who care
// can check for it.
var ErrEmptySchema = errors.New("category: empty attribute schema")

// ValidateAttributes checks that attrs conforms to schema (both raw JSON).
// Empty schema → ErrEmptySchema (caller decides what to do; usually: accept).
// Validation failure → *ValidationError with per-field messages.
func ValidateAttributes(schema, attrs json.RawMessage) error {
	if len(schema) == 0 || string(schema) == "{}" {
		return ErrEmptySchema
	}
	if len(attrs) == 0 {
		attrs = json.RawMessage("{}")
	}

	schemaLoader := gojsonschema.NewBytesLoader(schema)
	docLoader := gojsonschema.NewBytesLoader(attrs)
	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	if err != nil {
		return fmt.Errorf("schema validator: %w", err)
	}
	if result.Valid() {
		return nil
	}

	ve := &ValidationError{
		Summary: "attributes don't match this category's schema",
		Fields:  map[string]string{},
	}
	for _, desc := range result.Errors() {
		field := desc.Field()
		if field == "(root)" {
			field = "_"
		}
		ve.Fields[field] = desc.Description()
	}
	return ve
}
