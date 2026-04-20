package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/auth"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/config"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/migrations"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/server"
	"github.com/MRobbinsSAI/collection-tracker/api/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.FromEnv()
	logger.Info("startup", "env", cfg.Env, "addr", cfg.HTTPAddr)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startupCancel()

	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	if err := migrations.Up(startupCtx, cfg.DatabaseURL); err != nil {
		logger.Error("migrations failed", "err", err)
		os.Exit(1)
	}

	pool, err := store.NewPool(startupCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("db pool failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if rows, err := seed.Categories(startupCtx, pool); err != nil {
		logger.Error("category seed failed", "err", err)
		os.Exit(1)
	} else {
		logger.Info("category seed applied", "rows", rows)
	}

	sessionSecret, err := resolveSessionSecret(cfg, logger)
	if err != nil {
		logger.Error("session secret resolution failed", "err", err)
		os.Exit(1)
	}
	isProd := cfg.Env == "production"
	sessions := auth.NewManager(sessionSecret, isProd)

	users := store.NewUsers(pool)
	googleCfg := auth.GoogleConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
	}
	if !googleCfg.Configured() {
		logger.Warn("google oauth not configured; /auth/google/* will return 503 until set")
	}
	googleAuth := auth.NewGoogle(googleCfg, users, sessions, cfg.WebBaseURL, logger)

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: server.NewRouter(server.Deps{
			Logger:      logger,
			DBPinger:    pool,
			Categories:  store.NewCategories(pool),
			Users:       users,
			Sessions:    sessions,
			GoogleAuth:  googleAuth,
			CORSOrigins: cfg.CORSOrigins,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown initiated")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}

// resolveSessionSecret prefers an explicit SESSION_SECRET. If missing in
// development we generate a throwaway secret per process, which invalidates
// sessions on restart but keeps local dev unblocked. In production we refuse
// to start without an explicit 32+ byte secret.
func resolveSessionSecret(cfg config.Config, logger *slog.Logger) ([]byte, error) {
	if cfg.SessionSecret != "" {
		if len(cfg.SessionSecret) < 32 {
			return nil, errors.New("SESSION_SECRET must be at least 32 bytes")
		}
		return []byte(cfg.SessionSecret), nil
	}
	if cfg.Env == "production" {
		return nil, errors.New("SESSION_SECRET is required in production")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	logger.Warn("SESSION_SECRET not set; generated an ephemeral secret. Sessions will not survive restarts.")
	return b, nil
}
