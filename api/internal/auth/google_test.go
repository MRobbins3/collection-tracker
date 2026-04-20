package auth

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

type fakeUsers struct {
	seenSub   string
	seenEmail string
	seenName  string
	ret       store.User
	retErr    error
}

func (f *fakeUsers) UpsertByGoogleSub(_ context.Context, sub, email, name string) (store.User, error) {
	f.seenSub, f.seenEmail, f.seenName = sub, email, name
	if f.retErr != nil {
		return store.User{}, f.retErr
	}
	if f.ret.ID == "" {
		return store.User{ID: "user-" + sub, GoogleSub: sub, Email: email, DisplayName: name}, nil
	}
	return f.ret, nil
}

func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestManager() *Manager {
	return NewManager([]byte("01234567890123456789012345678901"), false)
}

func TestStartReturns503WhenNotConfigured(t *testing.T) {
	g := NewGoogle(GoogleConfig{}, &fakeUsers{}, newTestManager(), "http://web", newDiscardLogger())
	req := httptest.NewRequest(http.MethodGet, "/auth/google/start", nil)
	rr := httptest.NewRecorder()
	g.Start(rr, req)
	require.Equal(t, http.StatusServiceUnavailable, rr.Code)
}

func TestStartRedirectsAndSetsStateCookie(t *testing.T) {
	g := NewGoogle(GoogleConfig{
		ClientID:     "cid",
		ClientSecret: "secret",
		RedirectURL:  "http://api/callback",
	}, &fakeUsers{}, newTestManager(), "http://web", newDiscardLogger())

	req := httptest.NewRequest(http.MethodGet, "/auth/google/start", nil)
	rr := httptest.NewRecorder()
	g.Start(rr, req)

	require.Equal(t, http.StatusFound, rr.Code)
	loc, err := url.Parse(rr.Header().Get("Location"))
	require.NoError(t, err)
	require.Contains(t, loc.Host, "accounts.google.com")
	require.Equal(t, "cid", loc.Query().Get("client_id"))
	require.Equal(t, "http://api/callback", loc.Query().Get("redirect_uri"))

	var state string
	for _, c := range rr.Result().Cookies() {
		if c.Name == oauthStateCookieName {
			state = c.Value
		}
	}
	require.NotEmpty(t, state, "expected state cookie to be set")
	require.Equal(t, state, loc.Query().Get("state"))
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	g := NewGoogle(GoogleConfig{ClientID: "c", ClientSecret: "s", RedirectURL: "http://api/cb"},
		&fakeUsers{}, newTestManager(), "http://web", newDiscardLogger())

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=x&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "right"})
	rr := httptest.NewRecorder()
	g.Callback(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

// TestCallbackHappyPath wires fake Google token + userinfo endpoints into the
// oauth client to exercise the full Callback path end-to-end: state verify,
// code exchange, profile fetch, user upsert, session issuance, redirect.
func TestCallbackHappyPath(t *testing.T) {
	// Fake token endpoint
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-access","token_type":"Bearer","expires_in":3600}`))
	}))
	t.Cleanup(tokenSrv.Close)

	// Fake userinfo endpoint
	userinfoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer fake-access", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sub":            "google-sub-xyz",
			"email":          "alice@example.com",
			"email_verified": true,
			"name":           "Alice Example",
		})
	}))
	t.Cleanup(userinfoSrv.Close)

	users := &fakeUsers{}
	mgr := newTestManager()
	g := NewGoogle(GoogleConfig{ClientID: "c", ClientSecret: "s", RedirectURL: "http://api/cb"},
		users, mgr, "http://web", newDiscardLogger())

	// Point the oauth client at our fake endpoints.
	g.oauth.Endpoint = oauth2.Endpoint{
		AuthURL:  tokenSrv.URL + "/auth",
		TokenURL: tokenSrv.URL + "/token",
	}
	g.userinfoURL = userinfoSrv.URL

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?code=abc&state=matching", nil)
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "matching"})
	rr := httptest.NewRecorder()
	g.Callback(rr, req)

	require.Equal(t, http.StatusFound, rr.Code)
	require.True(t, strings.HasPrefix(rr.Header().Get("Location"), "http://web/"))

	require.Equal(t, "google-sub-xyz", users.seenSub)
	require.Equal(t, "alice@example.com", users.seenEmail)
	require.Equal(t, "Alice Example", users.seenName)

	// Confirm a session cookie was written and decodes to the expected user.
	var sessionCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookieName {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie, "expected session cookie to be set")

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.AddCookie(sessionCookie)
	s, err := mgr.Read(req2)
	require.NoError(t, err)
	require.Equal(t, "user-google-sub-xyz", s.UserID)
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	g := NewGoogle(GoogleConfig{}, &fakeUsers{}, newTestManager(), "http://web", newDiscardLogger())
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rr := httptest.NewRecorder()
	g.Logout(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var cleared *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookieName {
			cleared = c
		}
	}
	require.NotNil(t, cleared)
	require.Less(t, cleared.MaxAge, 0)
}

func TestBestNameFallbacks(t *testing.T) {
	cases := []struct {
		name    string
		profile googleProfile
		want    string
	}{
		{"uses name", googleProfile{Name: "Alice Example", GivenName: "Alice", Email: "a@x"}, "Alice Example"},
		{"falls back to given_name", googleProfile{GivenName: "Alice", Email: "a@x"}, "Alice"},
		{"falls back to email local", googleProfile{Email: "alice@example.com"}, "alice"},
		{"ultimate fallback", googleProfile{}, "Collector"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.profile.BestName())
		})
	}
}
