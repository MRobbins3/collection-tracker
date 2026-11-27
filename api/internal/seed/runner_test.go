package seed_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
)

// Dev seeds are allowlisted to development and test. Every other APP_ENV is
// refused before the database is touched, so a nil pool is fine here:
// reaching it would panic and fail the test.
func TestApplyRefusesDevSeedsOutsideDevelopmentAndTest(t *testing.T) {
	for _, env := range []string{"", "production", "prod", "Production", "staging", "Development", " development"} {
		t.Run("env="+env, func(t *testing.T) {
			applied, err := seed.Apply(context.Background(), nil, seed.Options{Env: env, Dev: true})
			require.ErrorIs(t, err, seed.ErrDevSeedNotAllowed)
			require.Empty(t, applied)
		})
	}
}

func TestDevSeedsAllowedInDevelopmentAndTest(t *testing.T) {
	for _, env := range []string{"development", "test"} {
		require.True(t, seed.DevSeedsAllowed(env), env)
	}
}
