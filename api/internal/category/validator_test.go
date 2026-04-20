package category_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
)

func TestValidateAttributes(t *testing.T) {
	lego := json.RawMessage(`{
		"type": "object",
		"properties": {
			"set_number": {"type": "string"},
			"piece_count": {"type": "integer", "minimum": 0}
		},
		"additionalProperties": false
	}`)

	cases := []struct {
		name        string
		schema      string
		attrs       string
		wantEmpty   bool
		wantInvalid bool
		wantField   string
	}{
		{
			name:   "happy",
			schema: string(lego),
			attrs:  `{"set_number": "75192", "piece_count": 7541}`,
		},
		{
			name:        "wrong type",
			schema:      string(lego),
			attrs:       `{"set_number": 75192, "piece_count": 7541}`,
			wantInvalid: true,
			wantField:   "set_number",
		},
		{
			name:        "negative integer violates minimum",
			schema:      string(lego),
			attrs:       `{"piece_count": -1}`,
			wantInvalid: true,
			wantField:   "piece_count",
		},
		{
			name:        "unknown property rejected",
			schema:      string(lego),
			attrs:       `{"set_number": "1", "piece_count": 1, "sneaky": "x"}`,
			wantInvalid: true,
		},
		{
			name:      "empty schema returns ErrEmptySchema",
			schema:    `{}`,
			attrs:     `{"anything": "goes"}`,
			wantEmpty: true,
		},
		{
			name:   "empty attrs treated as empty object and valid",
			schema: string(lego),
			attrs:  ``,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := category.ValidateAttributes(json.RawMessage(tc.schema), json.RawMessage(tc.attrs))
			switch {
			case tc.wantEmpty:
				require.ErrorIs(t, err, category.ErrEmptySchema)
			case tc.wantInvalid:
				var ve *category.ValidationError
				require.True(t, errors.As(err, &ve), "expected *ValidationError, got %T: %v", err, err)
				require.NotEmpty(t, ve.Fields)
				if tc.wantField != "" {
					_, ok := ve.Fields[tc.wantField]
					require.True(t, ok, "expected error on %q, got fields %+v", tc.wantField, ve.Fields)
				}
			default:
				require.NoError(t, err)
			}
		})
	}
}
