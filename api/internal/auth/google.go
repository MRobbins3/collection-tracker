package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// GoogleConfig carries the credentials needed to talk to Google. When
// ClientID or ClientSecret is empty, the handlers respond 503 — we want the
// app to run in local dev without Google set up.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Configured reports whether the handler has enough to initiate an OAuth flow.
func (c GoogleConfig) Configured() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

// UserStore is the narrow view the OAuth callback needs. Defined here so
// tests don't have to pull pgx into scope.
type UserStore interface {
	UpsertByGoogleSub(ctx context.Context, sub, email, displayName string) (store.User, error)
}

// Google implements the three OAuth endpoints (start/callback/logout).
type Google struct {
	cfg         GoogleConfig
	oauth       *oauth2.Config
	users       UserStore
	sessions    *Manager
	webBaseURL  string
	userinfoURL string
	logger      *slog.Logger
	now         func() time.Time
}

// NewGoogle constructs the handler. userinfoURL can be overridden in tests.
func NewGoogle(cfg GoogleConfig, users UserStore, sessions *Manager, webBaseURL string, logger *slog.Logger) *Google {
	return &Google{
		cfg: cfg,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     googleoauth.Endpoint,
		},
		users:       users,
		sessions:    sessions,
		webBaseURL:  strings.TrimRight(webBaseURL, "/"),
		userinfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
		logger:      logger,
		now:         time.Now,
	}
}

// Start begins the OAuth handshake. Stores a random `state` in a short-lived
// cookie, then redirects the browser to Google's consent screen.
func (g *Google) Start(w http.ResponseWriter, r *http.Request) {
	if !g.cfg.Configured() {
		http.Error(w,
			`{"error":"google oauth not configured (set GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET, GOOGLE_OAUTH_REDIRECT_URL)"}`,
			http.StatusServiceUnavailable)
		return
	}

	state, err := randomState()
	if err != nil {
		g.logger.Error("oauth start: random state", "err", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   int(oauthStateMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   g.sessions.secure,
		SameSite: http.SameSiteLaxMode,
	})

	url := g.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline, oauth2.ApprovalForce)
	http.Redirect(w, r, url, http.StatusFound)
}

// Callback is invoked by Google after consent. Verifies the state, exchanges
// the code, fetches the user's Google profile, upserts the user, issues a
// session cookie, and sends the browser back to the web app.
func (g *Google) Callback(w http.ResponseWriter, r *http.Request) {
	if !g.cfg.Configured() {
		http.Error(w, `{"error":"google oauth not configured"}`, http.StatusServiceUnavailable)
		return
	}

	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		g.logger.Warn("oauth callback reported error", "err", errParam)
		g.redirectToWeb(w, r, "/?auth=cancelled")
		return
	}

	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		http.Error(w, `{"error":"missing code or state"}`, http.StatusBadRequest)
		return
	}

	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || stateCookie.Value == "" {
		http.Error(w, `{"error":"missing state cookie"}`, http.StatusBadRequest)
		return
	}
	if stateCookie.Value != state {
		http.Error(w, `{"error":"state mismatch"}`, http.StatusBadRequest)
		return
	}
	// one-shot: clear it
	http.SetCookie(w, &http.Cookie{
		Name:   oauthStateCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	token, err := g.oauth.Exchange(r.Context(), code)
	if err != nil {
		g.logger.Warn("oauth exchange failed", "err", err)
		http.Error(w, `{"error":"code exchange failed"}`, http.StatusBadGateway)
		return
	}

	profile, err := g.fetchProfile(r.Context(), token)
	if err != nil {
		g.logger.Warn("oauth userinfo fetch failed", "err", err)
		http.Error(w, `{"error":"userinfo fetch failed"}`, http.StatusBadGateway)
		return
	}

	user, err := g.users.UpsertByGoogleSub(r.Context(), profile.Sub, profile.Email, profile.BestName())
	if err != nil {
		g.logger.Error("user upsert failed", "err", err)
		http.Error(w, `{"error":"user upsert failed"}`, http.StatusInternalServerError)
		return
	}

	if err := g.sessions.Write(w, Session{UserID: user.ID}); err != nil {
		g.logger.Error("session write failed", "err", err)
		http.Error(w, `{"error":"session write failed"}`, http.StatusInternalServerError)
		return
	}

	g.logger.Info("user signed in", "user_id", user.ID, "email", user.Email)
	g.redirectToWeb(w, r, "/?auth=ok")
}

// Logout clears the session cookie. Idempotent — no error if no session was
// present to begin with.
func (g *Google) Logout(w http.ResponseWriter, _ *http.Request) {
	g.sessions.Clear(w)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (g *Google) redirectToWeb(w http.ResponseWriter, r *http.Request, path string) {
	if g.webBaseURL == "" {
		// fall back to a JSON OK — useful in tests where we can assert on it
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}
	http.Redirect(w, r, g.webBaseURL+path, http.StatusFound)
}

type googleProfile struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
}

func (p googleProfile) BestName() string {
	switch {
	case p.Name != "":
		return p.Name
	case p.GivenName != "":
		return p.GivenName
	case p.Email != "":
		if i := strings.IndexByte(p.Email, '@'); i > 0 {
			return p.Email[:i]
		}
		return p.Email
	default:
		return "Collector"
	}
}

func (g *Google) fetchProfile(ctx context.Context, token *oauth2.Token) (googleProfile, error) {
	client := g.oauth.Client(ctx, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.userinfoURL, nil)
	if err != nil {
		return googleProfile{}, fmt.Errorf("new request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return googleProfile{}, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return googleProfile{}, fmt.Errorf("userinfo status %d", resp.StatusCode)
	}
	var p googleProfile
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return googleProfile{}, fmt.Errorf("decode: %w", err)
	}
	if p.Sub == "" {
		return googleProfile{}, errors.New("userinfo missing sub")
	}
	return p, nil
}

const (
	oauthStateCookieName = "ct_oauth_state"
	oauthStateMaxAge     = 5 * time.Minute
)

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
