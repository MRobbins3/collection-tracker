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

	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

// Deps groups the dependencies the HTTP layer needs. A struct keeps the
// constructor signature stable as we add handlers.
type Deps struct {
	Logger      *slog.Logger
	DBPinger    DBPinger
	Categories  CategoryStore
	CORSOrigins []string
}

// DBPinger is satisfied by *pgxpool.Pool; declared here so handler tests can
// swap in a fake without pulling pgx into their imports.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// CategoryStore is the narrow view of category queries the HTTP layer needs.
type CategoryStore interface {
	List(ctx context.Context) ([]store.Category, error)
	Search(ctx context.Context, q string) ([]store.Category, error)
	GetBySlug(ctx context.Context, slug string) (store.Category, error)
}

type handlers struct {
	logger     *slog.Logger
	db         DBPinger
	categories CategoryStore
}

// NewRouter returns the fully-wired chi router. Constructing it in one place
// means handler tests exercise the same middleware stack that runs in
// production.
func NewRouter(deps Deps) http.Handler {
	h := &handlers{
		logger:     deps.Logger,
		db:         deps.DBPinger,
		categories: deps.Categories,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(echoRequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(deps.Logger))
	if len(deps.CORSOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   deps.CORSOrigins,
			AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"X-Request-Id"},
			AllowCredentials: true,
			MaxAge:           300,
		}))
	}

	r.Get("/healthz", h.healthz)
	r.Get("/readyz", h.readyz)
	r.Get("/categories", h.listCategories)
	r.Get("/categories/{slug}", h.getCategory)
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
