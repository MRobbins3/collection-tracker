package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: server.NewRouter(server.Deps{
			Logger:     logger,
			DBPinger:   pool,
			Categories: store.NewCategories(pool),
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
