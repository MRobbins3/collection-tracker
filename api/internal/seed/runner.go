package seed

import (
	"context"
	"errors"
	"io/fs"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrDevSeedInProduction is returned when dev seed data is requested while
// APP_ENV=production. Fake users must never reach a real database.
var ErrDevSeedInProduction = errors.New("refusing to apply dev seed data when APP_ENV=production")

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
	return &Runner{FS: fstest.MapFS{}}
}

// Apply runs the seed sets selected by opts and returns the applied file
// paths in order.
func (r *Runner) Apply(_ context.Context, _ *pgxpool.Pool, _ Options) ([]string, error) {
	return nil, errors.New("not implemented")
}

// Apply runs the embedded seed files. See Runner.Apply.
func Apply(ctx context.Context, pool *pgxpool.Pool, opts Options) ([]string, error) {
	return New().Apply(ctx, pool, opts)
}
