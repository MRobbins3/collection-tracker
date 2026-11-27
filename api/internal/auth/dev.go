package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"regexp"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// DevUserStore is the narrow view the dev sign-in provider needs.
type DevUserStore interface {
	ListDevPersonas(ctx context.Context) ([]store.Persona, error)
	GetByGoogleSub(ctx context.Context, sub string) (store.User, error)
}

// Dev is the local/CI-only sign-in provider (ADR 0008): it signs the browser
// in as one of the persona users from the dev seed set, with no Google round
// trip. main constructs it only when DEV_AUTH_ENABLED=true and APP_ENV is
// development or test; otherwise its routes are never registered (404).
//
// Login writes the same session cookie as the Google callback, so everything
// downstream (/me, auth.Require, logout) is identical for both providers.
type Dev struct {
	users          DevUserStore
	sessions       *Manager
	allowedOrigins map[string]struct{}
	logger         *slog.Logger
}

// NewDev builds the provider. allowedOrigins is the API's CORS allowlist;
// a login request carrying any other Origin is refused.
func NewDev(users DevUserStore, sessions *Manager, allowedOrigins []string, logger *slog.Logger) *Dev {
	return &Dev{users: users, sessions: sessions, allowedOrigins: originSet(allowedOrigins), logger: logger}
}

// Personas lists the users the picker offers.
func (d *Dev) Personas(w http.ResponseWriter, r *http.Request) {
	personas, err := d.users.ListDevPersonas(r.Context())
	if err != nil {
		d.logger.Error("dev personas: list failed", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if personas == nil {
		personas = []store.Persona{}
	}
	writeAuthJSON(w, http.StatusOK, map[string]any{"personas": personas})
}

// handlePattern matches the part of a persona's google_sub after "dev:".
var handlePattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// Login signs in as the persona named in a JSON body {"persona": "<handle>"}.
//
// CSRF: the session cookie is SameSite=Lax, which stops cross-site requests
// from riding an existing session, but login CSRF (an attacker signing the
// victim in as someone else) sets a new cookie, so it needs its own guard:
// checkJSONAndOrigin.
func (d *Dev) Login(w http.ResponseWriter, r *http.Request) {
	if !checkJSONAndOrigin(w, r, d.allowedOrigins) {
		return
	}

	var body struct {
		Persona string `json:"persona"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if !handlePattern.MatchString(body.Persona) {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "persona is required"})
		return
	}

	user, err := d.users.GetByGoogleSub(r.Context(), store.DevSubPrefix+body.Persona)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "unknown test user"})
			return
		}
		d.logger.Error("dev login: lookup failed", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if err := d.sessions.Write(w, Session{UserID: user.ID}); err != nil {
		d.logger.Error("dev login: session write failed", "err", err)
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	d.logger.Info("dev user signed in", "user_id", user.ID, "persona", body.Persona)
	writeAuthJSON(w, http.StatusOK, map[string]any{"user": user})
}

func originSet(allowed []string) map[string]struct{} {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return set
}

// checkJSONAndOrigin is the CSRF guard for session-changing POSTs (dev
// login, logout). It writes the error response and returns false when the
// request fails either layer:
//   - Content-Type must be application/json (415). HTML forms can only send
//     urlencoded, multipart, or text/plain, and a cross-origin fetch with a
//     JSON body triggers a CORS preflight that our allowlist rejects.
//   - When the browser sends an Origin header, it must be on the CORS
//     allowlist (403). Requests without Origin (curl, tests) are allowed.
func checkJSONAndOrigin(w http.ResponseWriter, r *http.Request, allowedOrigins map[string]struct{}) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAuthJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "content type must be application/json"})
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		if _, ok := allowedOrigins[origin]; !ok {
			writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "origin not allowed"})
			return false
		}
	}
	return true
}

func writeAuthJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
