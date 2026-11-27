// Package profile holds the rules for the user-editable profile fields:
// handle, display name, and bio, plus the deterministic avatar color.
// Handlers validate with these before writing; the database repeats the
// handle format as a CHECK constraint (migration 0003) as a backstop.
package profile

import (
	"hash/fnv"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/textnorm"
)

// Handle rules: 3–30 characters, lowercase letters, digits, and underscores,
// starting with a letter. Handles are public, appear in URLs (/u/<handle>),
// and are always stored lowercase, so uniqueness is case-insensitive.
const (
	HandleMinLen = 3
	HandleMaxLen = 30
	// derivedBaseMaxLen leaves room for a numeric collision suffix.
	derivedBaseMaxLen = 24
	fallbackHandle    = "collector"

	DisplayNameMaxLen = 50
	BioMaxLen         = 160
)

// User-facing validation messages, shared with tests so copy changes are
// deliberate.
const (
	MsgHandleLength      = "Handles are 3–30 characters."
	MsgHandleChars       = "Use only lowercase letters, numbers, and underscores."
	MsgHandleStart       = "Start your handle with a letter."
	MsgHandleReserved    = "That handle is reserved. Try another."
	MsgHandleTaken       = "That handle is taken. Try another."
	MsgDisplayNameLength = "Display name must be 1–50 characters."
	MsgBioLength         = "Bio must be 160 characters or fewer."
)

// reserved handles would collide with routes, impersonate staff, or confuse
// people. Keep in sync with the copy in migration 0003 (which only matters
// for the one-time backfill).
var reserved = map[string]struct{}{}

func init() {
	for _, h := range []string{
		"about", "account", "accounts", "admin", "administrator", "anonymous",
		"api", "app", "auth", "catalog", "categories", "category",
		"collection", "collections", "collection_tracker", "contact",
		"delete", "dev", "edit", "everyone", "follow", "followers",
		"following", "friend", "friends", "help", "home", "login", "logout",
		"me", "mod", "moderator", "my", "new", "null", "official", "owner",
		"privacy", "profile", "register", "root", "security", "settings",
		"signin", "signout", "signup", "staff", "support", "system", "terms",
		"u", "undefined", "user", "users", "www",
	} {
		reserved[h] = struct{}{}
	}
}

// IsReserved reports whether h (already normalized) is off-limits.
func IsReserved(h string) bool {
	_, ok := reserved[h]
	return ok
}

// NormalizeHandle is applied to every handle a user types: trim, drop one
// leading "@" (people paste "@liam"), and lowercase.
func NormalizeHandle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	return strings.ToLower(s)
}

// ValidateHandle returns "" when h (already normalized) is acceptable, or
// the message to show. It doesn't check whether the handle is taken.
func ValidateHandle(h string) string {
	if n := utf8.RuneCountInString(h); n < HandleMinLen || n > HandleMaxLen {
		return MsgHandleLength
	}
	for _, r := range h {
		if !isHandleRune(r) {
			return MsgHandleChars
		}
	}
	if h[0] < 'a' || h[0] > 'z' {
		return MsgHandleStart
	}
	if IsReserved(h) {
		return MsgHandleReserved
	}
	return ""
}

func isHandleRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_'
}

// DeriveHandleBase suggests a handle for an account that doesn't have one
// (sign-up, and the migration 0003 backfill, which mirrors this logic in
// SQL). It prefers the display name, which is already public, over the
// email address, which isn't; the email's local part is used only when the
// name yields nothing usable. The result is valid apart from possibly being
// reserved or taken; PickFreeHandle resolves both.
func DeriveHandleBase(displayName, email string) string {
	local, _, _ := strings.Cut(email, "@")
	for _, src := range []string{displayName, local} {
		if h := slugify(src); h != "" {
			return h
		}
	}
	return fallbackHandle
}

func slugify(s string) string {
	var b strings.Builder
	underscore := false
	for _, r := range textnorm.Fold(s) {
		if isHandleRune(r) && r != '_' {
			b.WriteRune(r)
			underscore = false
			continue
		}
		if !underscore && b.Len() > 0 {
			b.WriteByte('_')
			underscore = true
		}
	}
	h := strings.TrimRight(b.String(), "_")
	if len(h) < HandleMinLen {
		return ""
	}
	if h[0] < 'a' || h[0] > 'z' {
		h = "u_" + h
	}
	if len(h) > derivedBaseMaxLen {
		h = strings.TrimRight(h[:derivedBaseMaxLen], "_")
	}
	return h
}

// PickFreeHandle returns base if it's free and not reserved, otherwise the
// first of base2, base3, … that is. taken holds the existing handles that
// start with base.
func PickFreeHandle(base string, taken map[string]bool) string {
	if !taken[base] && !IsReserved(base) {
		return base
	}
	for n := 2; ; n++ {
		c := base + strconv.Itoa(n)
		if !taken[c] && !IsReserved(c) {
			return c
		}
	}
}

// NormalizeDisplayName trims and collapses whitespace. Casing is the user's
// choice (the settings page suggests Title Case but never forces it).
func NormalizeDisplayName(s string) string { return textnorm.Clean(s) }

// ValidateDisplayName checks a normalized display name.
func ValidateDisplayName(s string) string {
	if n := utf8.RuneCountInString(s); n < 1 || n > DisplayNameMaxLen {
		return MsgDisplayNameLength
	}
	return ""
}

// NormalizeBio keeps the bio to one tidy line.
func NormalizeBio(s string) string { return textnorm.Clean(s) }

// ValidateBio checks a normalized bio. Empty is fine.
func ValidateBio(s string) string {
	if utf8.RuneCountInString(s) > BioMaxLen {
		return MsgBioLength
	}
	return ""
}

// AvatarPalette is the set of color names the web maps to Tailwind classes
// (web/app/utils/avatar.ts). Append only: reordering repaints everyone.
var AvatarPalette = []string{
	"rose", "orange", "amber", "emerald", "teal", "sky", "indigo", "violet", "fuchsia", "slate",
}

// AvatarColor picks a stable palette color from the user's id, so a user's
// initials avatar keeps its color when they rename themselves.
func AvatarColor(userID string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(userID))
	return AvatarPalette[h.Sum32()%uint32(len(AvatarPalette))]
}
