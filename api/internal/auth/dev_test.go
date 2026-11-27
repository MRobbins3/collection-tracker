package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

type fakeDevUsers struct {
	personas []store.Persona
	listErr  error
	users    map[string]store.User // keyed by google_sub
	getErr   error
	seenSub  string
	created  []string // google_subs passed to UpsertByGoogleSub
}

func (f *fakeDevUsers) UpsertByGoogleSub(_ context.Context, sub, email, displayName string) (store.User, error) {
	f.created = append(f.created, sub)
	return store.User{ID: "user-" + sub, GoogleSub: sub, Email: email, DisplayName: displayName, Handle: "disposable_tester"}, nil
}

func (f *fakeDevUsers) ListDevPersonas(_ context.Context) ([]store.Persona, error) {
	return f.personas, f.listErr
}

func (f *fakeDevUsers) GetByGoogleSub(_ context.Context, sub string) (store.User, error) {
	f.seenSub = sub
	if f.getErr != nil {
		return store.User{}, f.getErr
	}
	u, ok := f.users[sub]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func newDevFixture() (*Dev, *fakeDevUsers, *Manager) {
	users := &fakeDevUsers{
		personas: []store.Persona{
			{Handle: "liam", DisplayName: "Liam Johnson", Email: "liam.johnson@example.com", CollectionCount: 2, ItemCount: 6},
			{Handle: "emma", DisplayName: "Emma Garcia", Email: "emma.garcia@example.com"},
		},
		users: map[string]store.User{
			"dev:liam": {ID: "user-liam", GoogleSub: "dev:liam", Email: "liam.johnson@example.com", DisplayName: "Liam Johnson"},
		},
	}
	mgr := newTestManager()
	return NewDev(users, mgr, []string{"http://localhost:3000"}, newDiscardLogger()), users, mgr
}

func loginRequest(body, contentType, origin string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/dev/login", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func sessionCookieFrom(rr *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

func TestDevPersonasListsSeededUsers(t *testing.T) {
	d, _, _ := newDevFixture()
	rr := httptest.NewRecorder()
	d.Personas(rr, httptest.NewRequest(http.MethodGet, "/auth/dev/personas", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	var body struct {
		Personas []store.Persona `json:"personas"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Personas, 2)
	require.Equal(t, "liam", body.Personas[0].Handle)
	require.Equal(t, 6, body.Personas[0].ItemCount)
}

func TestDevPersonasEmptyIsAnArrayNotNull(t *testing.T) {
	d, users, _ := newDevFixture()
	users.personas = nil
	rr := httptest.NewRecorder()
	d.Personas(rr, httptest.NewRequest(http.MethodGet, "/auth/dev/personas", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"personas":[]}`, rr.Body.String())
}

func TestDevPersonasStoreErrorIs500(t *testing.T) {
	d, users, _ := newDevFixture()
	users.listErr = errors.New("db down")
	rr := httptest.NewRecorder()
	d.Personas(rr, httptest.NewRequest(http.MethodGet, "/auth/dev/personas", nil))
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestDevLoginWritesTheSessionCookie(t *testing.T) {
	d, users, mgr := newDevFixture()
	rr := httptest.NewRecorder()
	d.Login(rr, loginRequest(`{"persona":"liam"}`, "application/json", "http://localhost:3000"))

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "dev:liam", users.seenSub, "handle maps to the dev: google_sub")

	var body struct {
		User store.User `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "Liam Johnson", body.User.DisplayName)

	c := sessionCookieFrom(rr)
	require.NotNil(t, c, "expected the same session cookie the Google flow writes")
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	s, err := mgr.Read(req)
	require.NoError(t, err)
	require.Equal(t, "user-liam", s.UserID)
}

func TestDevLoginAcceptsJSONWithCharsetAndNoOrigin(t *testing.T) {
	d, _, _ := newDevFixture()
	rr := httptest.NewRecorder()
	d.Login(rr, loginRequest(`{"persona":"liam"}`, "application/json; charset=utf-8", ""))
	require.Equal(t, http.StatusOK, rr.Code, "non-browser clients (curl, tests) send no Origin")
	require.NotNil(t, sessionCookieFrom(rr))
}

func TestDevLoginRejections(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		contentType string
		origin      string
		want        int
	}{
		// An HTML form can only send these three content types, so requiring
		// JSON blocks cross-site form posts (login CSRF).
		{"form post", "persona=liam", "application/x-www-form-urlencoded", "", http.StatusUnsupportedMediaType},
		{"multipart post", "x", "multipart/form-data; boundary=x", "", http.StatusUnsupportedMediaType},
		{"text/plain post", `{"persona":"liam"}`, "text/plain", "", http.StatusUnsupportedMediaType},
		{"missing content type", `{"persona":"liam"}`, "", "", http.StatusUnsupportedMediaType},
		{"foreign origin", `{"persona":"liam"}`, "application/json", "https://evil.example", http.StatusForbidden},
		{"malformed json", `{"persona":`, "application/json", "", http.StatusBadRequest},
		{"missing persona", `{}`, "application/json", "", http.StatusBadRequest},
		{"handle with a path", `{"persona":"../liam"}`, "application/json", "", http.StatusBadRequest},
		{"handle with the prefix", `{"persona":"dev:liam"}`, "application/json", "", http.StatusBadRequest},
		{"unknown persona", `{"persona":"nobody"}`, "application/json", "", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _, _ := newDevFixture()
			rr := httptest.NewRecorder()
			d.Login(rr, loginRequest(tc.body, tc.contentType, tc.origin))
			require.Equal(t, tc.want, rr.Code, rr.Body.String())
			require.Nil(t, sessionCookieFrom(rr), "a rejected login must not set a session")
		})
	}
}

func TestDevLoginStoreErrorIs500(t *testing.T) {
	d, users, _ := newDevFixture()
	users.getErr = errors.New("db down")
	rr := httptest.NewRecorder()
	d.Login(rr, loginRequest(`{"persona":"liam"}`, "application/json", ""))
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Nil(t, sessionCookieFrom(rr))
}

func disposableRequest(contentType, origin string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/dev/disposable", strings.NewReader(`{}`))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestDevDisposableCreatesAFreshUserAndSignsIn(t *testing.T) {
	d, users, mgr := newDevFixture()

	rr := httptest.NewRecorder()
	d.Disposable(rr, disposableRequest("application/json", "http://localhost:3000"))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	require.Len(t, users.created, 1)
	sub := users.created[0]
	require.True(t, strings.HasPrefix(sub, DisposableSubPrefix), sub)
	require.False(t, strings.HasPrefix(sub, store.DevSubPrefix),
		"disposable users must not show up in the persona picker")

	var body struct{ User store.User }
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "user-"+sub, body.User.ID)
	require.Contains(t, body.User.Email, "@example.com")

	c := sessionCookieFrom(rr)
	require.NotNil(t, c)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	s, err := mgr.Read(req)
	require.NoError(t, err)
	require.Equal(t, "user-"+sub, s.UserID)

	// Each call makes a different user.
	d.Disposable(httptest.NewRecorder(), disposableRequest("application/json", ""))
	require.Len(t, users.created, 2)
	require.NotEqual(t, users.created[0], users.created[1])
}

func TestDevDisposableHasTheLoginCSRFGuard(t *testing.T) {
	d, users, _ := newDevFixture()
	rr := httptest.NewRecorder()
	d.Disposable(rr, disposableRequest("text/plain", ""))
	require.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
	rr = httptest.NewRecorder()
	d.Disposable(rr, disposableRequest("application/json", "https://evil.example"))
	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Empty(t, users.created)
}
