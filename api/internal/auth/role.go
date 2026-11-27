package auth

import (
	"context"
	"net/http"
)

// RoleAdmin is the only role today (ADR 0004 "Roles and moderation"): an
// admin reviews catalog submissions and corrections. Most users have no
// role at all.
const RoleAdmin = "admin"

type roleCtxKey struct{}

// WithRole records the signed-in user's role on the context. Sessions hold
// only the user id (ADR 0003), so the role comes from the users table on
// each request: the server's live-user check, which already loads the
// user, calls this. A role removed in the database takes effect on the
// very next request.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleCtxKey{}, role)
}

// RoleFromContext returns the role WithRole stored, or "" for none.
func RoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(roleCtxKey{}).(string)
	return role
}

// RequireRole returns middleware that lets a request through only when the
// signed-in user has the role. Anonymous requests get 401 (like Require);
// signed-in users without the role get 403. Admin routes aren't anyone's
// own resource, so the 404-not-403 rule for cross-user access doesn't
// apply, and the web bundle names the admin pages anyway (ADR 0012). If
// nothing upstream recorded a role, the request is refused: it fails
// closed.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := FromContext(r.Context()); !ok {
				writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			if RoleFromContext(r.Context()) != role {
				writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
