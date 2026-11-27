package seed

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDevSeedInProduction is returned when dev seed data is requested while
// APP_ENV=production. Fake users must never reach a real database.
var ErrDevSeedInProduction = errors.New("refusing to apply dev seed data when APP_ENV=production")

// The seed sets, applied in this order. Reference data is always applied;
// dev data only when Options.Dev is set.
const (
	referenceDir = "reference"
	devDir       = "dev"
)

//go:embed sql
var embedded embed.FS

// Options controls which seed sets Apply runs.
type Options struct {
	// Env is the APP_ENV value ("development", "production", ...).
	Env string
	// Dev enables the dev seed set (SEED_DEV_DATA=true).
	Dev bool
}

// Runner applies the SQL seed files found under reference/ and dev/ in FS.
type Runner struct {
	FS fs.FS
}

// New returns a Runner over the seed files embedded in the binary.
func New() *Runner {
	sub, err := fs.Sub(embedded, "sql")
	if err != nil {
		// fs.Sub only fails on an invalid path literal; this is a programmer error.
		panic(err)
	}
	return &Runner{FS: sub}
}

// Apply runs every reference/*.sql file, then (when opts.Dev) every
// dev/*.sql file, each set in lexical filename order, inside one transaction.
// Every file must be idempotent: Apply runs on every API start. It returns
// the applied file paths in order.
func (r *Runner) Apply(ctx context.Context, pool *pgxpool.Pool, opts Options) ([]string, error) {
	if opts.Dev && opts.Env == "production" {
		return nil, ErrDevSeedInProduction
	}

	dirs := []string{referenceDir}
	if opts.Dev {
		dirs = append(dirs, devDir)
	}

	var files []string
	for _, dir := range dirs {
		names, err := sqlFiles(r.FS, dir)
		if err != nil {
			return nil, err
		}
		files = append(files, names...)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin seed tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, f := range files {
		body, err := fs.ReadFile(r.FS, f)
		if err != nil {
			return nil, fmt.Errorf("read seed %s: %w", f, err)
		}
		// No arguments, so pgx uses the simple protocol, which allows a file
		// to hold several statements.
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			return nil, fmt.Errorf("apply seed %s: %w", f, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit seed tx: %w", err)
	}
	return files, nil
}

// sqlFiles lists dir/*.sql in lexical order. A missing directory is an empty
// set, so a tree can ship without dev seeds.
func sqlFiles(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list seeds in %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		out = append(out, path.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

// Apply runs the embedded seed files. See Runner.Apply.
func Apply(ctx context.Context, pool *pgxpool.Pool, opts Options) ([]string, error) {
	return New().Apply(ctx, pool, opts)
}
