package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
)

func newManager(t *testing.T) *auth.Manager {
	t.Helper()
	// 32-byte dev secret; fine for tests.
	secret := []byte("01234567890123456789012345678901")
	return auth.NewManager(secret, false)
}

func TestWriteThenReadRoundtripsSession(t *testing.T) {
	m := newManager(t)

	rr := httptest.NewRecorder()
	require.NoError(t, m.Write(rr, auth.Session{UserID: "user-123"}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}

	got, err := m.Read(req)
	require.NoError(t, err)
	require.Equal(t, "user-123", got.UserID)
}

func TestReadWithoutCookieReturnsErrNoSession(t *testing.T) {
	m := newManager(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := m.Read(req)
	require.ErrorIs(t, err, auth.ErrNoSession)
}

func TestReadWithTamperedCookieReturnsErrNoSession(t *testing.T) {
	m := newManager(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "ct_session", Value: "not-a-valid-value"})
	_, err := m.Read(req)
	require.ErrorIs(t, err, auth.ErrNoSession)
}

func TestReadWithDifferentSecretReturnsErrNoSession(t *testing.T) {
	writer := newManager(t)
	reader := auth.NewManager([]byte("totally-different-32-byte-secret"), false)

	rr := httptest.NewRecorder()
	require.NoError(t, writer.Write(rr, auth.Session{UserID: "u1"}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	_, err := reader.Read(req)
	require.ErrorIs(t, err, auth.ErrNoSession)
}

func TestClearOverwritesCookieWithExpiredOne(t *testing.T) {
	m := newManager(t)
	rr := httptest.NewRecorder()
	m.Clear(rr)

	cookies := rr.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "ct_session", cookies[0].Name)
	require.Equal(t, "", cookies[0].Value)
	require.Less(t, cookies[0].MaxAge, 0)
}

func TestMiddlewarePlacesSessionOnContext(t *testing.T) {
	m := newManager(t)

	// Build a request that carries a valid session cookie.
	rr := httptest.NewRecorder()
	require.NoError(t, m.Write(rr, auth.Session{UserID: "user-42"}))

	var captured auth.Session
	var ok bool
	handler := m.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured, ok = auth.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, ok)
	require.Equal(t, "user-42", captured.UserID)
}

func TestMiddlewareNoCookieLeavesContextEmpty(t *testing.T) {
	m := newManager(t)
	var ok bool
	handler := m.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok = auth.FromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	require.False(t, ok)
}

func TestRequireReturns401WhenNoSession(t *testing.T) {
	called := false
	handler := auth.Require(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	require.False(t, called)
}

func TestRequireAllowsRequestWithSessionOnContext(t *testing.T) {
	m := newManager(t)
	rr := httptest.NewRecorder()
	require.NoError(t, m.Write(rr, auth.Session{UserID: "user-7"}))

	var called bool
	chain := m.Middleware(auth.Require(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	})))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
	out := httptest.NewRecorder()
	chain.ServeHTTP(out, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, out.Code)
}
