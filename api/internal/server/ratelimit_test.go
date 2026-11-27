package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestKeyedLimiter(t *testing.T) {
	clock := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	l := newKeyedLimiter(3, time.Second)
	l.now = func() time.Time { return clock }

	for i := 0; i < 3; i++ {
		require.True(t, l.Allow("a"), "burst %d", i)
	}
	require.False(t, l.Allow("a"), "burst spent")
	require.True(t, l.Allow("b"), "keys are independent")

	clock = clock.Add(time.Second)
	require.True(t, l.Allow("a"), "one token back per interval")
	require.False(t, l.Allow("a"))

	clock = clock.Add(time.Hour)
	for i := 0; i < 3; i++ {
		require.True(t, l.Allow("a"), "refills only up to burst")
	}
	require.False(t, l.Allow("a"))
}
