package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
)

// requestWith builds a request carrying an optional session and role, the
// way Manager.Middleware and the server's live-user check leave them.
func requestWith(t *testing.T, userID, role string) *http.Request {
	t.Helper()
	m := newManager(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/moderation", nil)
	if userID != "" {
		rr := httptest.NewRecorder()
		require.NoError(t, m.Write(rr, auth.Session{UserID: userID}))
		for _, c := range rr.Result().Cookies() {
			req.AddCookie(c)
		}
	}
	var out *http.Request
	m.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { out = r })).
		ServeHTTP(httptest.NewRecorder(), req)
	if role != "" {
		out = out.WithContext(auth.WithRole(out.Context(), role))
	}
	return out
}

func TestRequireRole(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := auth.RequireRole(auth.RoleAdmin)(ok)

	cases := []struct {
		name         string
		userID, role string
		want         int
	}{
		{"anonymous is unauthorized", "", "", http.StatusUnauthorized},
		{"signed in without a role is forbidden", "user-1", "", http.StatusForbidden},
		{"another role is forbidden", "user-1", "curator", http.StatusForbidden},
		{"admin passes", "user-1", auth.RoleAdmin, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, requestWith(t, tc.userID, tc.role))
			require.Equal(t, tc.want, rr.Code)
		})
	}
}

func TestRoleFromContext(t *testing.T) {
	req := requestWith(t, "user-1", "")
	require.Empty(t, auth.RoleFromContext(req.Context()), "no role unless something set one")
	req = requestWith(t, "user-1", auth.RoleAdmin)
	require.Equal(t, auth.RoleAdmin, auth.RoleFromContext(req.Context()))
}
