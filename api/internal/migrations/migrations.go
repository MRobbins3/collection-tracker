// Package migrations embeds the SQL migration files and exposes a runner
// backed by pressly/goose. The app and integration tests share this entry
// point so they exercise the same migration code path.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // register the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// Up applies all outstanding migrations.
func Up(ctx context.Context, dbURL string) error {
	return run(ctx, dbURL, func(db *sql.DB) error {
		return goose.UpContext(ctx, db, "sql")
	})
}

// Down rolls back the most recent migration. Intended for tests and local
// development — production rollbacks should be explicit and reviewed.
func Down(ctx context.Context, dbURL string) error {
	return run(ctx, dbURL, func(db *sql.DB) error {
		return goose.DownContext(ctx, db, "sql")
	})
}

// DownTo rolls back migrations until the target version is reached. `0` fully
// unwinds the schema, which is useful for integration test teardown.
func DownTo(ctx context.Context, dbURL string, version int64) error {
	return run(ctx, dbURL, func(db *sql.DB) error {
		return goose.DownToContext(ctx, db, "sql", version)
	})
}

func run(ctx context.Context, dbURL string, fn func(*sql.DB) error) error {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}

	goose.SetBaseFS(sqlFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	return fn(db)
}
