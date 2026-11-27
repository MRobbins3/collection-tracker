package textnorm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/textnorm"
)

func TestClean(t *testing.T) {
	cases := map[string]string{
		"":                  "",
		"   ":               "",
		"Star Wars":         "Star Wars",
		"  star   wars  ":   "star wars",
		"tab\tand\nnewline": "tab and newline",
		" nbsp   ":          "nbsp",
	}
	for in, want := range cases {
		require.Equal(t, want, textnorm.Clean(in), "%q", in)
	}
}

func TestKey(t *testing.T) {
	require.Equal(t, "star wars", textnorm.Key("  Star   WARS "))
	require.Equal(t, textnorm.Key("lego"), textnorm.Key("LEGO"))
}

func TestFold(t *testing.T) {
	cases := map[string]string{
		"José García": "jose garcia",
		"Zoë":         "zoe",
		"Ångström":    "angstrom",
		"plain":       "plain",
		"MIXED Case":  "mixed case",
	}
	for in, want := range cases {
		require.Equal(t, want, textnorm.Fold(in), in)
	}
}
