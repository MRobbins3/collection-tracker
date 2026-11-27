// Package server wires HTTP routes and middleware for the collection-tracker API.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/category"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// Deps groups the dependencies the HTTP layer needs. A struct keeps the
// constructor signature stable as we add handlers.
type Deps struct {
	Logger      *slog.Logger
	DBPinger    DBPinger
	Categories  CategoryStore
	Collections CollectionStore
	Items       ItemStore
	Catalog     CatalogStore
	Moderation  ModerationStore
	Overview    OverviewStore
	Users       UserStore
	// Relationships feeds the privacy rule (ADR 0009). nil falls back to
	// "self or stranger" (handler tests that don't care).
	Relationships RelationshipStore
	// Social is friends and followers (ADR 0014); the same
	// store.Relationships in production. nil leaves its routes unwired.
	Social     SocialStore
	Sessions   *auth.Manager
	GoogleAuth *auth.Google
	// DevAuth is the local/CI-only sign-in provider. nil (the default, and
	// always in production) means its /auth/dev/* routes don't exist.
	DevAuth     *auth.Dev
	CORSOrigins []string
}

// DBPinger is satisfied by *pgxpool.Pool; declared here so handler tests can
// swap in a fake without pulling pgx into their imports.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// CategoryStore is the narrow view of category queries the HTTP layer
// needs. viewerID is the caller ("" when anonymous): reads return approved
// categories plus the caller's own (ADR 0013).
type CategoryStore interface {
	List(ctx context.Context, viewerID string) ([]store.Category, error)
	Search(ctx context.Context, viewerID, q string) ([]store.Category, error)
	GetBySlug(ctx context.Context, viewerID, slug string) (store.Category, error)
	GetByID(ctx context.Context, id string) (store.Category, error)
	ListMine(ctx context.Context, userID string) ([]store.Category, error)
	ApprovedSources(ctx context.Context) ([]category.Source, error)
	Create(ctx context.Context, userID string, in store.CategoryInput) (store.Category, error)
	Update(ctx context.Context, userID, slug string, in store.CategoryInput) (store.Category, error)
}

// UserStore is the narrow view of user and profile queries the HTTP layer
// needs.
type UserStore interface {
	GetByID(ctx context.Context, id string) (store.User, error)
	GetProfile(ctx context.Context, userID string) (store.Profile, error)
	GetProfileByHandle(ctx context.Context, handle string) (store.Profile, error)
	UpdateProfile(ctx context.Context, userID string, p store.ProfilePatch) (store.Profile, error)
	HandleAvailable(ctx context.Context, handle, userID string) (bool, error)
	Delete(ctx context.Context, userID string) error
}

type handlers struct {
	logger        *slog.Logger
	db            DBPinger
	categories    CategoryStore
	collections   CollectionStore
	items         ItemStore
	catalog       CatalogStore
	moderation    ModerationStore
	overview      OverviewStore
	users         UserStore
	relationships RelationshipStore
	social        SocialStore
	handleChecks  *keyedLimiter
	requestLimits *keyedLimiter
	searchLimits  *keyedLimiter
	sessions      *auth.Manager
	googleAuth    *auth.Google
	devAuth       *auth.Dev
}

// NewRouter returns the fully-wired chi router. Constructing it in one place
// means handler tests exercise the same middleware stack that runs in
// production.
func NewRouter(deps Deps) http.Handler {
	h := &handlers{
		logger:        deps.Logger,
		db:            deps.DBPinger,
		categories:    deps.Categories,
		collections:   deps.Collections,
		items:         deps.Items,
		catalog:       deps.Catalog,
		moderation:    deps.Moderation,
		overview:      deps.Overview,
		users:         deps.Users,
		relationships: deps.Relationships,
		social:        deps.Social,
		handleChecks:  newKeyedLimiter(handleCheckBurst, handleCheckInterval),
		requestLimits: newKeyedLimiter(requestBurst, requestInterval),
		searchLimits:  newKeyedLimiter(searchBurst, searchInterval),
		sessions:      deps.Sessions,
		googleAuth:    deps.GoogleAuth,
		devAuth:       deps.DevAuth,
	}

	if h.relationships == nil {
		h.relationships = selfOrNone{}
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(echoRequestID)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(deps.Logger))
	if len(deps.CORSOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   deps.CORSOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"X-Request-Id"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}
	if deps.Sessions != nil {
		r.Use(deps.Sessions.Middleware)
	}

	// Public
	r.Get("/healthz", h.healthz)
	r.Get("/readyz", h.readyz)
	r.Get("/categories", h.listCategories)
	r.Get("/categories/{slug}", h.getCategory)
	r.Get("/category-fields", h.listCategoryFields)
	r.Get("/catalog/entries", h.listCatalog)
	r.Get("/catalog/entries/{entryID}", h.getCatalogEntry)
	// /me is intentionally public: it answers "who am I?" with 200 {user:null}
	// when anonymous, so app-boot calls never emit console 401s. Handlers that
	// *require* a session still return 401 via the auth.Require group below.
	r.Get("/me", h.me)
	// Public profiles. Signed-in is optional; the privacy rule decides what
	// each viewer sees, and hidden profiles are 404 (ADR 0009).
	r.Get("/users/{handle}", h.getPublicProfile)
	r.Get("/users/{handle}/collections/{id}", h.getPublicCollection)

	// Auth. /auth/providers tells the web which sign-in buttons to show; it
	// is separate from /me so /me's anonymous 200 {"user":null} contract
	// stays exactly as it was.
	r.Get("/auth/providers", h.authProviders)
	if deps.GoogleAuth != nil {
		r.Get("/auth/google/start", deps.GoogleAuth.Start)
		r.Get("/auth/google/callback", deps.GoogleAuth.Callback)
	}
	if deps.DevAuth != nil {
		r.Get("/auth/dev/personas", deps.DevAuth.Personas)
		r.Post("/auth/dev/login", deps.DevAuth.Login)
		r.Post("/auth/dev/disposable", deps.DevAuth.Disposable)
	}
	if deps.Sessions != nil {
		// Logout only clears the session cookie, so it serves every provider.
		// It requires JSON + an allowed Origin, like dev login (logout CSRF).
		r.Post("/auth/logout", deps.Sessions.LogoutHandler(deps.CORSOrigins))
	}

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(auth.Require)
		r.Use(h.requireLiveUser)
		r.Get("/me/collections", h.listMyCollections)
		r.Post("/me/collections", h.createMyCollection)
		r.Get("/me/collections/{id}", h.getMyCollection)
		r.Patch("/me/collections/{id}", h.renameMyCollection)
		r.Delete("/me/collections/{id}", h.deleteMyCollection)

		r.Get("/me/collections/{id}/items", h.listItems)
		r.Post("/me/collections/{id}/items", h.createItem)
		r.Patch("/me/collections/{id}/items/{itemID}", h.updateItem)
		r.Delete("/me/collections/{id}/items/{itemID}", h.deleteItem)
		r.Get("/me/collections/{id}/items/{itemID}/catalog-suggestions", h.suggestCatalogEntries)
		r.Put("/me/collections/{id}/items/{itemID}/catalog-entry", h.linkItemToCatalog)
		r.Delete("/me/collections/{id}/items/{itemID}/catalog-entry", h.unlinkItemFromCatalog)

		r.Get("/me/catalog-ownership", h.listMyCatalogOwnership)

		// Catalog suggestions and corrections (ADR 0012).
		r.Post("/me/catalog-submissions", h.submitCatalogEntry)
		r.Get("/me/catalog-submissions", h.listMySubmissions)
		r.Post("/me/catalog-corrections", h.proposeCatalogCorrection)

		r.Get("/me/items", h.searchMyItems)

		// User-built categories (ADR 0013).
		r.Get("/me/categories", h.listMyCategories)
		r.Post("/me/categories", h.createMyCategory)
		r.Patch("/me/categories/{slug}", h.updateMyCategory)
		r.Post("/me/categories/{slug}/submit", h.submitMyCategory)

		r.Get("/me/overview", h.getMyOverview)
		r.Post("/me/pinned-categories", h.pinMyCategory)
		r.Delete("/me/pinned-categories/{slug}", h.unpinMyCategory)
		r.Patch("/me/overview-settings", h.patchMyOverviewSettings)

		r.Get("/me/profile", h.getMyProfile)
		r.Patch("/me/profile", h.patchMyProfile)
		r.Get("/me/handle-availability", h.handleAvailability)
		r.Delete("/me", h.deleteMe)

		// Friends and followers (ADR 0014). New requests go to a profile
		// (/users/{handle}/...) and pass the privacy rule; answers and
		// removals live under /me and act only on your own relationships.
		if h.social != nil {
			sc := h.social
			r.Post("/users/{handle}/friend-request", h.sendFriendRequest)
			r.Delete("/users/{handle}/friend-request", h.removal(sc.CancelFriendRequest, "cancelFriendRequest"))
			r.Post("/users/{handle}/follow", h.requestFollow)
			r.Delete("/users/{handle}/follow", h.removal(sc.Unfollow, "unfollow"))
			r.Post("/me/friend-requests/{handle}/accept", h.approval(sc.AcceptFriendRequest, "acceptFriendRequest"))
			r.Post("/me/friend-requests/{handle}/decline", h.removal(sc.DeclineFriendRequest, "declineFriendRequest"))
			r.Delete("/me/friends/{handle}", h.removal(sc.Unfriend, "unfriend"))
			r.Post("/me/follow-requests/{handle}/approve", h.approval(sc.ApproveFollower, "approveFollower"))
			r.Post("/me/follow-requests/{handle}/deny", h.removal(sc.DenyFollower, "denyFollower"))
			r.Delete("/me/followers/{handle}", h.removal(sc.RemoveFollower, "removeFollower"))
			r.Get("/me/friends", h.listPeople(sc.ListFriends, "listFriends"))
			r.Get("/me/followers", h.listPeople(sc.ListFollowers, "listFollowers"))
			r.Get("/me/following", h.listPeople(sc.ListFollowing, "listFollowing"))
			r.Get("/me/requests", h.listMyRequests)
			r.Get("/me/requests/count", h.countMyRequests)
			r.Get("/people", h.searchPeople)
		}

		// Admin only (ADR 0004 roles, ADR 0012). requireLiveUser put the
		// user's current role on the context; non-admins get 403.
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireRole(auth.RoleAdmin))
			r.Get("/admin/moderation", h.listModerationQueue)
			r.Get("/admin/moderation/{requestID}", h.getModerationRequest)
			r.Post("/admin/moderation/{requestID}/approve", h.approveModerationRequest)
			r.Post("/admin/moderation/{requestID}/reject", h.rejectModerationRequest)
			r.Post("/admin/moderation/{requestID}/merge", h.mergeModerationRequest)
		})
	})

	return r
}

// echoRequestID copies the chi-generated request id onto the response so
// clients can quote it when reporting issues.
func echoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set("X-Request-Id", id)
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"dur_ms", time.Since(start).Milliseconds(),
				"req_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
