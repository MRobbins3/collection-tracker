package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func logoutRequest(contentType, origin string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", strings.NewReader(`{}`))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	newTestManager().LogoutHandler([]string{"http://localhost:3000"})(rr, logoutRequest("application/json", "http://localhost:3000"))

	require.Equal(t, http.StatusOK, rr.Code)
	cleared := sessionCookieFrom(rr)
	require.NotNil(t, cleared)
	require.Less(t, cleared.MaxAge, 0)
}

// Same guard as dev login: a cross-site form post must not be able to sign
// the victim out (logout CSRF).
func TestLogoutRejections(t *testing.T) {
	cases := []struct {
		name, contentType, origin string
		want                      int
	}{
		{"form post", "application/x-www-form-urlencoded", "", http.StatusUnsupportedMediaType},
		{"text/plain post", "text/plain", "", http.StatusUnsupportedMediaType},
		{"missing content type", "", "", http.StatusUnsupportedMediaType},
		{"foreign origin", "application/json", "https://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			newTestManager().LogoutHandler([]string{"http://localhost:3000"})(rr, logoutRequest(tc.contentType, tc.origin))
			require.Equal(t, tc.want, rr.Code)
			require.Nil(t, sessionCookieFrom(rr), "a rejected logout must not touch the session")
		})
	}
}
