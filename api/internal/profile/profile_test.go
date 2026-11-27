package profile_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/profile"
)

func TestNormalizeHandle(t *testing.T) {
	cases := map[string]string{
		"liam":         "liam",
		"  Liam ":      "liam",
		"@Liam_J":      "liam_j",
		"@@x":          "@x",
		"LIAM JOHNSON": "liam johnson", // still invalid; validation reports it
	}
	for in, want := range cases {
		require.Equal(t, want, profile.NormalizeHandle(in), in)
	}
}

func TestValidateHandle(t *testing.T) {
	ok := []string{"liam", "liam_johnson", "abc", "a12", strings.Repeat("a", 30), "x_y_z"}
	for _, h := range ok {
		require.Empty(t, profile.ValidateHandle(h), h)
	}
	bad := map[string]string{
		"":                      profile.MsgHandleLength,
		"ab":                    profile.MsgHandleLength,
		strings.Repeat("a", 31): profile.MsgHandleLength,
		"liam johnson":          profile.MsgHandleChars,
		"liam-johnson":          profile.MsgHandleChars,
		"liam.johnson":          profile.MsgHandleChars,
		"Liam":                  profile.MsgHandleChars, // callers normalize first
		"josé":                  profile.MsgHandleChars,
		"1liam":                 profile.MsgHandleStart,
		"_liam":                 profile.MsgHandleStart,
		"admin":                 profile.MsgHandleReserved,
		"me":                    profile.MsgHandleLength, // length wins over reserved
		"settings":              profile.MsgHandleReserved,
		"support":               profile.MsgHandleReserved,
		strings.Repeat("é", 10): profile.MsgHandleChars,
		"abc" + "\u0000":        profile.MsgHandleChars,
		"null":                  profile.MsgHandleReserved,
		"collection_tracker":    profile.MsgHandleReserved,
		"administrator":         profile.MsgHandleReserved,
		"moderator":             profile.MsgHandleReserved,
	}
	for h, want := range bad {
		require.Equal(t, want, profile.ValidateHandle(h), "%q", h)
	}
}

func TestIsReserved(t *testing.T) {
	for _, h := range []string{"admin", "me", "settings", "api", "auth", "u", "my", "users"} {
		require.True(t, profile.IsReserved(h), h)
	}
	require.False(t, profile.IsReserved("liam"))
}

func TestDeriveHandleBase(t *testing.T) {
	cases := []struct {
		name, email, want string
	}{
		{"Liam Johnson", "liam.johnson@example.com", "liam_johnson"},
		{"José García", "jg@example.com", "jose_garcia"},
		{"  O'Brien,   Pat  ", "x@example.com", "o_brien_pat"},
		{"42", "fan.club@example.com", "fan_club"},     // display name too short → email
		{"R2", "r2@example.com", "collector"},          // both too short → fallback
		{"3PO Fan", "x@example.com", "u_3po_fan"},      // must start with a letter
		{"渡辺", "watanabe.k@example.com", "watanabe_k"}, // nothing ASCII left → email
		{"An Extremely Long Display Name Indeed", "e@x.com", "an_extremely_long_displa"},
		{"Admin", "admin@example.com", "admin"}, // reserved is handled by the suffix step
	}
	for _, c := range cases {
		got := profile.DeriveHandleBase(c.name, c.email)
		require.Equal(t, c.want, got, "%q / %q", c.name, c.email)
		require.LessOrEqual(t, len(got), 24)
	}
}

func TestPickFreeHandle(t *testing.T) {
	taken := func(hs ...string) map[string]bool {
		m := map[string]bool{}
		for _, h := range hs {
			m[h] = true
		}
		return m
	}
	require.Equal(t, "liam", profile.PickFreeHandle("liam", taken()))
	require.Equal(t, "liam2", profile.PickFreeHandle("liam", taken("liam")))
	require.Equal(t, "liam4", profile.PickFreeHandle("liam", taken("liam", "liam2", "liam3")))
	require.Equal(t, "admin2", profile.PickFreeHandle("admin", taken()), "reserved base gets a suffix")
	got := profile.PickFreeHandle("abc", taken())
	require.Empty(t, profile.ValidateHandle(got))
}

func TestDisplayName(t *testing.T) {
	require.Equal(t, "Liam Johnson", profile.NormalizeDisplayName("  Liam   Johnson "))
	require.Empty(t, profile.ValidateDisplayName("Liam Johnson"))
	require.Empty(t, profile.ValidateDisplayName(strings.Repeat("é", 50)), "length counts characters, not bytes")
	require.Equal(t, profile.MsgDisplayNameLength, profile.ValidateDisplayName(""))
	require.Equal(t, profile.MsgDisplayNameLength, profile.ValidateDisplayName(strings.Repeat("a", 51)))
}

func TestBio(t *testing.T) {
	require.Equal(t, "Lego fan. Vinyl too.", profile.NormalizeBio("  Lego fan.\n\nVinyl   too. "))
	require.Empty(t, profile.ValidateBio(""))
	require.Empty(t, profile.ValidateBio(strings.Repeat("é", 160)))
	require.Equal(t, profile.MsgBioLength, profile.ValidateBio(strings.Repeat("a", 161)))
}

func TestAvatarColorIsDeterministicAndFromThePalette(t *testing.T) {
	a := profile.AvatarColor("d0000000-0000-4000-8000-000000000001")
	require.Equal(t, a, profile.AvatarColor("d0000000-0000-4000-8000-000000000001"))
	require.Contains(t, profile.AvatarPalette, a)

	seen := map[string]bool{}
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		seen[profile.AvatarColor(id)] = true
	}
	require.Greater(t, len(seen), 3, "colors should spread across the palette")
}
