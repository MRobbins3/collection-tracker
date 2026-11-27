package server

import (
	"sync"
	"time"
)

// keyedLimiter is a small in-memory token bucket per key (here: the
// signed-in user id). Each key holds up to burst tokens and regains one
// every interval. It's per process, which is enough for one API instance;
// behind several instances each would allow its own budget.
type keyedLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	burst    float64
	interval time.Duration
	now      func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

// maxBuckets bounds memory: past it, buckets that have fully refilled are
// dropped (they behave identically to a fresh bucket).
const maxBuckets = 10_000

func newKeyedLimiter(burst int, interval time.Duration) *keyedLimiter {
	return &keyedLimiter{
		buckets:  map[string]*bucket{},
		burst:    float64(burst),
		interval: interval,
		now:      time.Now,
	}
}

// Allow spends one token for key, reporting false when none are left.
func (l *keyedLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= maxBuckets {
			l.prune(now)
		}
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+float64(now.Sub(b.last))/float64(l.interval))
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (l *keyedLimiter) prune(now time.Time) {
	for k, b := range l.buckets {
		if b.tokens+float64(now.Sub(b.last))/float64(l.interval) >= l.burst {
			delete(l.buckets, k)
		}
	}
}
