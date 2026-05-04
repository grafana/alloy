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
	defer c.Close()
	for i := uint64(0); i < 600_000; i++ {
		c.Add(i, labels.FromStrings("k", "v"))
	}
	require.Equal(t, 100_000, c.Len())
}

func TestLRURelabelCache_BasicOps(t *testing.T) {
	c, err := newRelabelCache(Arguments{CacheSize: 100}, newTestEvictionsCounter())
	require.NoError(t, err)
	defer c.Close()
	require.IsType(t, &lruRelabelCache{}, c)

	lbls := labels.FromStrings("a", "b")
	_, ok := c.Get(1)
	require.False(t, ok)

	c.Add(1, lbls)
	got, ok := c.Get(1)
	require.True(t, ok)
	require.Equal(t, lbls, got)
	require.Equal(t, 1, c.Len())

	c.Remove(1)
	_, ok = c.Get(1)
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
		defer c.Close()

		lbls := labels.FromStrings("a", "b")
		c.Add(1, lbls)

		// Past the TTL with no scan in between.
		time.Sleep(2 * time.Minute)
		got, ok := c.Get(1)
		require.True(t, ok, "Get returns cached value until scan prunes it")
		require.Equal(t, lbls, got)
		require.Equal(t, 1, c.Len())
	})
}

func TestTTLRelabelCache_ScanEvicts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		evictions := newTestEvictionsCounter()
		c := newTTLRelabelCache(time.Minute, evictions)
		defer c.Close()

		c.Add(1, labels.FromStrings("a", "b"))
		c.Add(2, labels.FromStrings("c", "d"))
		require.Equal(t, 2, c.Len())

		time.Sleep(2 * time.Minute)
		c.scan(time.Now())

		require.Equal(t, 0, c.Len())
		require.Equal(t, float64(2), counterVal(t, evictions))
	})
}

func TestTTLRelabelCache_ScanLeavesFreshEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		evictions := newTestEvictionsCounter()
		c := newTTLRelabelCache(time.Minute, evictions)
		defer c.Close()

		c.Add(1, labels.FromStrings("old", ""))
		time.Sleep(90 * time.Second)
		c.Add(2, labels.FromStrings("fresh", ""))
		c.scan(time.Now())

		require.Equal(t, 1, c.Len())
		_, ok := c.Get(2)
		require.True(t, ok)
		require.Equal(t, float64(1), counterVal(t, evictions))
	})
}

func TestTTLRelabelCache_StaleRemove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Hour, newTestEvictionsCounter())
		defer c.Close()

		c.Add(1, labels.FromStrings("a", "b"))
		c.Remove(1)

		_, ok := c.Get(1)
		require.False(t, ok, "Remove must drop the entry immediately, even with a long TTL")
		require.Equal(t, 0, c.Len())
	})
}

// TestTTLRelabelCache_GetSlidesExpiry asserts that a successful Get
// pushes the entry's expiry forward by a full TTL, so series that
// stay active stay cached without ever paying for re-relabeling.
func TestTTLRelabelCache_GetSlidesExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Minute, newTestEvictionsCounter())
		defer c.Close()

		c.Add(1, labels.FromStrings("a", "b"))

		// Halfway through the TTL: still a hit, and the Get slides
		// expiry from t=60s out to t=90s.
		time.Sleep(30 * time.Second)
		_, ok := c.Get(1)
		require.True(t, ok)

		// Now at t=80s — past the original t=60s expiry. Without
		// sliding, this would be a miss. With sliding, the entry's
		// expiry is t=90s, so it's still valid.
		time.Sleep(50 * time.Second)
		_, ok = c.Get(1)
		require.True(t, ok, "Get must slide the TTL window forward")
	})
}

// TestTTLRelabelCache_ScanLeavesActiveEntries confirms the scanner
// does not evict an entry whose Get keeps refreshing it, even when
// scans fire after each TTL window's nominal expiry.
func TestTTLRelabelCache_ScanLeavesActiveEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		evictions := newTestEvictionsCounter()
		c := newTTLRelabelCache(time.Minute, evictions)
		defer c.Close()

		c.Add(1, labels.FromStrings("a", "b"))

		// Three full TTL cycles: each cycle, Get keeps the entry hot,
		// then scan runs after the original-expiry would have hit.
		for cycle := 0; cycle < 3; cycle++ {
			time.Sleep(45 * time.Second)
			_, ok := c.Get(1)
			require.True(t, ok)
			c.scan(time.Now())
		}

		require.Equal(t, 1, c.Len())
		require.Equal(t, float64(0), counterVal(t, evictions))
	})
}
