// Package auth owns authentication: signed-cookie sessions and the Google
// OAuth handshake. Sessions hold only the user id; everything else is
// re-fetched from the store on each request.
package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

// Session is the minimal authenticated-request payload.
type Session struct {
	UserID string `json:"uid"`
}

// ErrNoSession is returned by Manager.Read when no valid session cookie is
// present on the request. Handlers that require auth should treat this as 401.
var ErrNoSession = errors.New("auth: no session")

type sessionCtxKey struct{}

// Manager encodes/decodes sessions as signed-and-encrypted cookies and
// provides middleware that places the decoded session on the request context.
type Manager struct {
	codec      *securecookie.SecureCookie
	cookieName string
	maxAge     time.Duration
	secure     bool
}

// NewManager builds a Manager from a secret. Secret must be 32 or 64 bytes.
// For local dev, `secure` should be false (browsers drop Secure cookies on
// plain HTTP). In any hosted environment, secure must be true.
func NewManager(secret []byte, secure bool) *Manager {
	c := securecookie.New(secret, nil)
	c.SetSerializer(securecookie.JSONEncoder{})
	c.MaxAge(int(sessionMaxAge.Seconds()))
	return &Manager{
		codec:      c,
		cookieName: sessionCookieName,
		maxAge:     sessionMaxAge,
		secure:     secure,
	}
}

const (
	sessionCookieName = "ct_session"
	sessionMaxAge     = 30 * 24 * time.Hour
)

// Write encodes the session onto the response as an HTTP-only cookie.
func (m *Manager) Write(w http.ResponseWriter, s Session) error {
	encoded, err := m.codec.Encode(m.cookieName, s)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   int(m.maxAge.Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// Read returns the session from the request cookie or ErrNoSession.
func (m *Manager) Read(r *http.Request) (Session, error) {
	c, err := r.Cookie(m.cookieName)
	if err != nil || c.Value == "" {
		return Session{}, ErrNoSession
	}
	var s Session
	if err := m.codec.Decode(m.cookieName, c.Value, &s); err != nil {
		return Session{}, ErrNoSession
	}
	if s.UserID == "" {
		return Session{}, ErrNoSession
	}
	return s, nil
}

// Clear deletes the session cookie.
func (m *Manager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// Middleware places the session (if any) on the request context. It NEVER
// rejects requests itself — handlers that require a session call Require.
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, err := m.Read(r); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), sessionCtxKey{}, s))
		}
		next.ServeHTTP(w, r)
	})
}

// FromContext returns the session stored by Middleware, if any.
func FromContext(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionCtxKey{}).(Session)
	return s, ok && s.UserID != ""
}

// Require returns an http.Handler middleware that responds 401 when no valid
// session is on the context.
func Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := FromContext(r.Context()); !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
