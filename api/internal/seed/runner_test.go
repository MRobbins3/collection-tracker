package seed_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/MRobbinsSAI/collection-tracker/api/internal/seed"
)

// Production must refuse dev seeds before touching the database, so a nil
// pool is fine here: reaching it would panic and fail the test.
func TestApplyRefusesDevSeedsInProduction(t *testing.T) {
	applied, err := seed.Apply(context.Background(), nil, seed.Options{Env: "production", Dev: true})
	require.ErrorIs(t, err, seed.ErrDevSeedInProduction)
	require.Empty(t, applied)
}
