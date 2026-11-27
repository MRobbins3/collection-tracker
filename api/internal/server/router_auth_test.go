package server_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbins3/collection-tracker/api/internal/auth"
	"github.com/MRobbins3/collection-tracker/api/internal/server"
	"github.com/MRobbins3/collection-tracker/api/internal/store"
)

type fakeDevUsers struct{}

var liam = store.User{ID: "user-liam", GoogleSub: "dev:liam", Email: "liam.johnson@example.com", DisplayName: "Liam Johnson"}

func (fakeDevUsers) ListDevPersonas(_ context.Context) ([]store.Persona, error) {
	return []store.Persona{{Handle: "liam", DisplayName: liam.DisplayName, Email: liam.Email}}, nil
}

func (fakeDevUsers) UpsertByGoogleSub(_ context.Context, sub, email, name string) (store.User, error) {
	return store.User{ID: "user-" + sub, GoogleSub: sub, Email: email, DisplayName: name}, nil
}

func (fakeDevUsers) GetByGoogleSub(_ context.Context, sub string) (store.User, error) {
	if sub == liam.GoogleSub {
		return liam, nil
	}
	return store.User{}, store.ErrNotFound
}

type authRouterOpts struct {
	dev    bool
	google auth.GoogleConfig
}

func newAuthRouter(opts authRouterOpts) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessions := auth.NewManager([]byte("01234567890123456789012345678901"), false)
	origins := []string{"http://localhost:3000"}
	deps := server.Deps{
		Logger:      logger,
		DBPinger:    fakePinger{},
		Categories:  &fakeCategories{},
		Users:       &fakeUsers{user: liam},
		Sessions:    sessions,
		GoogleAuth:  auth.NewGoogle(opts.google, nil, sessions, "http://localhost:3000", logger),
		CORSOrigins: origins,
	}
	if opts.dev {
		deps.DevAuth = auth.NewDev(fakeDevUsers{}, sessions, origins, logger)
	}
	return server.NewRouter(deps)
}

func serve(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func devLoginReq(persona string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/dev/login", strings.NewReader(`{"persona":"`+persona+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:3000")
	return req
}

func TestDevAuthRoutesDoNotExistWhenDisabled(t *testing.T) {
	r := newAuthRouter(authRouterOpts{})
	require.Equal(t, http.StatusNotFound, serve(r, httptest.NewRequest(http.MethodGet, "/auth/dev/personas", nil)).Code)
	rr := serve(r, devLoginReq("liam"))
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Empty(t, rr.Result().Cookies(), "no session when dev auth is off")
}

func TestDevDisposableRouteOnlyWithDevAuth(t *testing.T) {
	req := func() *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/auth/dev/disposable", strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		return r
	}
	require.Equal(t, http.StatusNotFound, serve(newAuthRouter(authRouterOpts{}), req()).Code)
	require.Equal(t, http.StatusOK, serve(newAuthRouter(authRouterOpts{dev: true}), req()).Code)
}

func TestDevAuthRoutesWhenEnabled(t *testing.T) {
	r := newAuthRouter(authRouterOpts{dev: true})

	rr := serve(r, httptest.NewRequest(http.MethodGet, "/auth/dev/personas", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"handle":"liam"`)

	require.Equal(t, http.StatusMethodNotAllowed,
		serve(r, httptest.NewRequest(http.MethodGet, "/auth/dev/login?persona=liam", nil)).Code,
		"login is POST-only")
}

// The full round trip through the real middleware stack: dev login sets the
// cookie, /me sees the user, logout clears it, /me is anonymous again.
func TestDevLoginThenMeThenLogout(t *testing.T) {
	r := newAuthRouter(authRouterOpts{dev: true})

	rr := serve(r, devLoginReq("liam"))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	cookies := rr.Result().Cookies()
	require.NotEmpty(t, cookies)

	me := httptest.NewRequest(http.MethodGet, "/me", nil)
	for _, c := range cookies {
		me.AddCookie(c)
	}
	rr = serve(r, me)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"display_name":"Liam Johnson"`)

	logout := httptest.NewRequest(http.MethodPost, "/auth/logout", strings.NewReader(`{}`))
	logout.Header.Set("Content-Type", "application/json")
	logout.Header.Set("Origin", "http://localhost:3000")
	rr = serve(r, logout)
	require.Equal(t, http.StatusOK, rr.Code)
	var cleared *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == cookies[0].Name {
			cleared = c
		}
	}
	require.NotNil(t, cleared)
	require.Less(t, cleared.MaxAge, 0)
}

func TestLogoutRejectsCrossSiteFormPost(t *testing.T) {
	r := newAuthRouter(authRouterOpts{dev: true})
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", strings.NewReader("x=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusUnsupportedMediaType, serve(r, req).Code)
}

func TestDevLoginCORSPreflightOnlyForAllowedOrigin(t *testing.T) {
	r := newAuthRouter(authRouterOpts{dev: true})
	preflight := func(origin string) string {
		req := httptest.NewRequest(http.MethodOptions, "/auth/dev/login", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "POST")
		req.Header.Set("Access-Control-Request-Headers", "content-type")
		return serve(r, req).Header().Get("Access-Control-Allow-Origin")
	}
	require.Equal(t, "http://localhost:3000", preflight("http://localhost:3000"))
	require.Empty(t, preflight("https://evil.example"))
}

func TestAuthProviders(t *testing.T) {
	configured := auth.GoogleConfig{ClientID: "c", ClientSecret: "s", RedirectURL: "http://api/cb"}
	cases := []struct {
		name string
		opts authRouterOpts
		want string
	}{
		{"neither", authRouterOpts{}, `{"google":false,"dev":false}`},
		{"dev only", authRouterOpts{dev: true}, `{"google":false,"dev":true}`},
		{"google only", authRouterOpts{google: configured}, `{"google":true,"dev":false}`},
		{"both", authRouterOpts{dev: true, google: configured}, `{"google":true,"dev":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := serve(newAuthRouter(tc.opts), httptest.NewRequest(http.MethodGet, "/auth/providers", nil))
			require.Equal(t, http.StatusOK, rr.Code)
			require.JSONEq(t, tc.want, rr.Body.String())
		})
	}
}

// Adding providers must not change the anonymous /me contract.
func TestMeStaysAnonymous200WithDevAuthOn(t *testing.T) {
	rr := serve(newAuthRouter(authRouterOpts{dev: true}), httptest.NewRequest(http.MethodGet, "/me", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"user":null}`, rr.Body.String())
}
