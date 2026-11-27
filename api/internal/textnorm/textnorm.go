// Package textnorm is the shared write-time normalizer for user-entered text
// (see "Normalize user-entered values at write time" in CLAUDE.md). Anything
// users type that others may see, pick, or match on goes through Clean before
// it's stored; uniqueness and matching compare Key values.
package textnorm

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Clean trims and collapses every run of whitespace (including newlines and
// non-breaking spaces) to a single space. Case is preserved.
func Clean(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Key is the case-insensitive comparison form of s: Clean, then lowercase.
func Key(s string) string {
	return strings.ToLower(Clean(s))
}

// Fold is Key with diacritics removed ("José" → "jose"), for deriving ASCII
// identifiers such as handles from names.
func Fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return Key(out)
}
