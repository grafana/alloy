package relabel

import (
	"testing"
	"testing/synctest"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

func newTestEvictionsCounter() prometheus_client.Counter {
	return prometheus_client.NewCounter(prometheus_client.CounterOpts{Name: "test_evictions"})
}

func counterVal(t *testing.T, c prometheus_client.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return *m.Counter.Value
}

func TestLRURelabelCache_FillsToCap(t *testing.T) {
	c, err := newRelabelCache(Arguments{CacheSize: 100_000}, newTestEvictionsCounter())
	require.NoError(t, err)
	defer c.close()
	for i := uint64(0); i < 600_000; i++ {
		c.add(i, labels.FromStrings("k", "v"))
	}
	require.Equal(t, 100_000, c.len())
}

func TestLRURelabelCache_BasicOps(t *testing.T) {
	c, err := newRelabelCache(Arguments{CacheSize: 100}, newTestEvictionsCounter())
	require.NoError(t, err)
	defer c.close()
	require.IsType(t, &lruRelabelCache{}, c)

	lbls := labels.FromStrings("a", "b")
	_, ok := c.get(1)
	require.False(t, ok)

	c.add(1, lbls)
	got, ok := c.get(1)
	require.True(t, ok)
	require.Equal(t, lbls, got)
	require.Equal(t, 1, c.len())

	c.remove(1)
	_, ok = c.get(1)
	require.False(t, ok)
}

// TestTTLRelabelCache_GetReturnsCachedValuePastExpiry confirms that
// scan is the sole arbiter of eviction: a Get on an entry whose
// nominal expiry has passed but which scan hasn't pruned still returns
// a hit (and slides the entry forward). The cached value is always
// correct since relabel rules can't change without rebuilding the
// cache, so there's no reason to force a re-relabel on the caller.
func TestTTLRelabelCache_GetReturnsCachedValuePastExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Minute, newTestEvictionsCounter())
		defer c.close()

		lbls := labels.FromStrings("a", "b")
		c.add(1, lbls)

		// Past the TTL with no scan in between.
		time.Sleep(2 * time.Minute)
		got, ok := c.get(1)
		require.True(t, ok, "Get returns cached value until scan prunes it")
		require.Equal(t, lbls, got)
		require.Equal(t, 1, c.len())
	})
}

func TestTTLRelabelCache_ScanEvicts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		evictions := newTestEvictionsCounter()
		c := newTTLRelabelCache(time.Minute, evictions)
		defer c.close()

		c.add(1, labels.FromStrings("a", "b"))
		c.add(2, labels.FromStrings("c", "d"))
		require.Equal(t, 2, c.len())

		time.Sleep(2 * time.Minute)
		c.scan(time.Now())

		require.Equal(t, 0, c.len())
		require.Equal(t, float64(2), counterVal(t, evictions))
	})
}

func TestTTLRelabelCache_ScanLeavesFreshEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		evictions := newTestEvictionsCounter()
		c := newTTLRelabelCache(time.Minute, evictions)
		defer c.close()

		c.add(1, labels.FromStrings("old", ""))
		time.Sleep(90 * time.Second)
		c.add(2, labels.FromStrings("fresh", ""))
		c.scan(time.Now())

		require.Equal(t, 1, c.len())
		_, ok := c.get(2)
		require.True(t, ok)
		require.Equal(t, float64(1), counterVal(t, evictions))
	})
}

func TestTTLRelabelCache_StaleRemove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Hour, newTestEvictionsCounter())
		defer c.close()

		c.add(1, labels.FromStrings("a", "b"))
		c.remove(1)

		_, ok := c.get(1)
		require.False(t, ok, "Remove must drop the entry immediately, even with a long TTL")
		require.Equal(t, 0, c.len())
	})
}

// TestTTLRelabelCache_GetSlidesExpiry asserts that a successful Get
// pushes the entry's expiry forward by a full TTL, so series that
// stay active stay cached without ever paying for re-relabeling.
func TestTTLRelabelCache_GetSlidesExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Minute, newTestEvictionsCounter())
		defer c.close()

		c.add(1, labels.FromStrings("a", "b"))

		// Halfway through the TTL: still a hit, and the Get slides
		// expiry from t=60s out to t=90s.
		time.Sleep(30 * time.Second)
		_, ok := c.get(1)
		require.True(t, ok)

		// Now at t=80s — past the original t=60s expiry. Without
		// sliding, this would be a miss. With sliding, the entry's
		// expiry is t=90s, so it's still valid.
		time.Sleep(50 * time.Second)
		_, ok = c.get(1)
		require.True(t, ok, "Get must slide the TTL window forward")
	})
}

func TestShouldShrink(t *testing.T) {
	cases := []struct {
		name string
		peak int
		live int
		want bool
	}{
		// Below 1M peak: never shrink — absolute savings too small.
		{"under tier: huge drop on small peak", 100_000, 1_000, false},
		{"under tier: at boundary", 999_999, 0, false},

		// 1M-10M tier: 2x drop required.
		{"1M tier: 50% drop triggers", 1_000_000, 500_000, true},
		{"1M tier: just under 50% drop", 1_000_000, 500_001, false},
		{"1M tier: 40% drop does not trigger", 1_000_000, 600_000, false},

		// >=10M tier: 1.5x drop (33%) triggers.
		{"10M tier: 33% drop triggers", 10_000_000, 6_666_666, true},
		{"10M tier: 30% drop does not trigger", 10_000_000, 7_000_000, false},
		{"10M tier: 50% drop triggers", 10_000_000, 5_000_000, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldShrink(tc.peak, tc.live))
		})
	}
}

// TestTTLRelabelCache_ScanRebuildsAfterDrain verifies that the scanner
// rebuilds the underlying map after a large transient working set
// drains. We can't observe map bucket memory directly, but we can
// observe peak resetting to len(entries), which is the policy's
// internal trigger.
func TestTTLRelabelCache_ScanRebuildsAfterDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Minute, newTestEvictionsCounter())
		defer c.close()

		// Fill past the 1M-tier boundary, then let everything expire
		// except a small live set. Peak should reset to the post-shrink
		// size.
		const peak = 1_000_000
		for i := uint64(0); i < peak; i++ {
			c.add(i, labels.FromStrings("k", "v"))
		}
		require.Equal(t, peak, c.peak)

		// Keep a few entries fresh; let the rest expire.
		const keep = 100
		time.Sleep(30 * time.Second)
		for i := uint64(0); i < keep; i++ {
			_, ok := c.get(i)
			require.True(t, ok)
		}
		time.Sleep(45 * time.Second) // total 75s — past TTL for the rest

		c.scan(time.Now())

		require.Equal(t, keep, c.len())
		require.Equal(t, keep, c.peak, "peak resets to live size after rebuild")
	})
}

// TestTTLRelabelCache_ScanRebuildsAfterRemoveDrain verifies the
// shrink path fires for drains caused by Remove (stale markers) and
// not just by TTL expiry. The phase-1 RLock peeks at the shrink
// condition specifically for this case, since with no expirations to
// prune scan would otherwise early-return before reaching the rebuild.
func TestTTLRelabelCache_ScanRebuildsAfterRemoveDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Hour, newTestEvictionsCounter())
		defer c.close()

		const peak = 1_000_000
		for i := uint64(0); i < peak; i++ {
			c.add(i, labels.FromStrings("k", "v"))
		}
		require.Equal(t, peak, c.peak)

		// Drain via Remove — no expirations. With a 1h TTL nothing
		// would expire on its own; only the shrink-condition peek
		// drives the rebuild here.
		const keep = 100
		for i := uint64(keep); i < peak; i++ {
			c.remove(i)
		}
		require.Equal(t, keep, c.len())
		require.Equal(t, peak, c.peak, "peak preserved until scan rebuilds")

		c.scan(time.Now())

		require.Equal(t, keep, c.len())
		require.Equal(t, keep, c.peak, "peak resets after rebuild even without expirations")
	})
}

// TestTTLRelabelCache_ScanSkipsShrinkBelowTier confirms small caches
// don't pay the rebuild cost — peak stays at its original high-water
// mark even after a deep drain.
func TestTTLRelabelCache_ScanSkipsShrinkBelowTier(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Minute, newTestEvictionsCounter())
		defer c.close()

		const peak = 10_000 // below the 1M tier
		for i := uint64(0); i < peak; i++ {
			c.add(i, labels.FromStrings("k", "v"))
		}
		require.Equal(t, peak, c.peak)

		time.Sleep(2 * time.Minute) // expire all
		c.scan(time.Now())

		require.Equal(t, 0, c.len())
		require.Equal(t, peak, c.peak, "peak preserved — below shrink tier")
	})
}

// TestTTLRelabelCache_ScanLeavesActiveEntries confirms the scanner
// does not evict an entry whose Get keeps refreshing it, even when
// scans fire after each TTL window's nominal expiry.
func TestTTLRelabelCache_ScanLeavesActiveEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		evictions := newTestEvictionsCounter()
		c := newTTLRelabelCache(time.Minute, evictions)
		defer c.close()

		c.add(1, labels.FromStrings("a", "b"))

		// Three full TTL cycles: each cycle, Get keeps the entry hot,
		// then scan runs after the original-expiry would have hit.
		for cycle := 0; cycle < 3; cycle++ {
			time.Sleep(45 * time.Second)
			_, ok := c.get(1)
			require.True(t, ok)
			c.scan(time.Now())
		}

		require.Equal(t, 1, c.len())
		require.Equal(t, float64(0), counterVal(t, evictions))
	})
}
