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

func newTestTTLCounters() ttlCounters {
	return ttlCounters{
		evictions: prometheus_client.NewCounter(prometheus_client.CounterOpts{Name: "test_evictions"}),
		rebuilds:  prometheus_client.NewCounter(prometheus_client.CounterOpts{Name: "test_rebuilds"}),
	}
}

func counterVal(t *testing.T, c prometheus_client.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return *m.Counter.Value
}

func TestLRURelabelCache_FillsToCap(t *testing.T) {
	c := newRelabelCache(Arguments{CacheSize: 100_000}, newTestTTLCounters())
	defer c.Close()
	for i := uint64(0); i < 600_000; i++ {
		c.Add(i, labels.FromStrings("k", "v"))
	}
	require.Equal(t, 100_000, c.Len())
}

func TestLRURelabelCache_BasicOps(t *testing.T) {
	c := newRelabelCache(Arguments{CacheSize: 100}, newTestTTLCounters())
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

func TestTTLRelabelCache_LazyExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newRelabelCache(Arguments{CacheTTL: 30 * time.Second}, newTestTTLCounters()).(*ttlRelabelCache)
		defer c.Close()

		lbls := labels.FromStrings("a", "b")
		c.Add(1, lbls)

		// Within TTL: hit.
		got, ok := c.Get(1)
		require.True(t, ok)
		require.Equal(t, lbls, got)

		// Advance past TTL: lazy expiry rejects the entry on Get even
		// though Scan hasn't run yet.
		time.Sleep(31 * time.Second)
		_, ok = c.Get(1)
		require.False(t, ok)
		// Entry is still in the map until Scan prunes it.
		require.Equal(t, 1, c.Len())
	})
}

func TestTTLRelabelCache_ScanEvicts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		counters := newTestTTLCounters()
		c := newTTLRelabelCache(time.Minute, counters)
		defer c.Close()

		c.Add(1, labels.FromStrings("a", "b"))
		c.Add(2, labels.FromStrings("c", "d"))
		require.Equal(t, 2, c.Len())

		time.Sleep(2 * time.Minute)
		c.scan(time.Now())

		require.Equal(t, 0, c.Len())
		require.Equal(t, float64(2), counterVal(t, counters.evictions))
	})
}

func TestTTLRelabelCache_ScanLeavesFreshEntries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		counters := newTestTTLCounters()
		c := newTTLRelabelCache(time.Minute, counters)
		defer c.Close()

		c.Add(1, labels.FromStrings("old", ""))
		time.Sleep(90 * time.Second)
		c.Add(2, labels.FromStrings("fresh", ""))
		c.scan(time.Now())

		require.Equal(t, 1, c.Len())
		_, ok := c.Get(2)
		require.True(t, ok)
		require.Equal(t, float64(1), counterVal(t, counters.evictions))
	})
}

func TestTTLRelabelCache_StaleRemove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := newTTLRelabelCache(time.Hour, newTestTTLCounters())
		defer c.Close()

		c.Add(1, labels.FromStrings("a", "b"))
		c.Remove(1)

		_, ok := c.Get(1)
		require.False(t, ok, "Remove must drop the entry immediately, even with a long TTL")
		require.Equal(t, 0, c.Len())
	})
}

func TestTTLRelabelCache_RebuildAfterShrink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Tighten the rebuild knobs so the test can assert on them
		// without inserting tens of thousands of entries.
		t.Cleanup(func() {
			rebuildFloor = 4096
			rebuildSpacing = 30 * time.Minute
		})
		rebuildFloor = 16
		rebuildSpacing = time.Minute

		counters := newTestTTLCounters()
		c := newTTLRelabelCache(time.Minute, counters)

		// Spike the cache well above the rebuild floor.
		for i := uint64(0); i < 100; i++ {
			c.Add(i, labels.FromStrings("k", "v"))
		}
		require.Equal(t, 100, c.Len())

		// Let the cache age out and scan; the scan should both prune
		// the entries and rebuild the underlying map because the live
		// count fell below half the watermark.
		time.Sleep(2 * time.Minute)
		c.scan(time.Now())

		require.Equal(t, 0, c.Len())
		require.Equal(t, float64(1), counterVal(t, counters.rebuilds))
	})
}

func TestTTLRelabelCache_RebuildRateLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t.Cleanup(func() {
			rebuildFloor = 4096
			rebuildSpacing = 30 * time.Minute
		})
		rebuildFloor = 16
		rebuildSpacing = 30 * time.Minute

		counters := newTestTTLCounters()
		c := newTTLRelabelCache(time.Minute, counters)

		for i := uint64(0); i < 100; i++ {
			c.Add(i, labels.FromStrings("k", "v"))
		}

		// First scan after expiry — rebuilds.
		time.Sleep(2 * time.Minute)
		c.scan(time.Now())
		require.Equal(t, float64(1), counterVal(t, counters.rebuilds))

		// Spike again, age out again, but call Scan before
		// rebuildSpacing has elapsed: the cache should NOT rebuild.
		for i := uint64(100); i < 200; i++ {
			c.Add(i, labels.FromStrings("k", "v"))
		}
		time.Sleep(2 * time.Minute)
		c.scan(time.Now())
		require.Equal(t, float64(1), counterVal(t, counters.rebuilds), "rebuild rate-limited")

		// After spacing elapses, the next eligible scan rebuilds again.
		for i := uint64(200); i < 300; i++ {
			c.Add(i, labels.FromStrings("k", "v"))
		}
		time.Sleep(31 * time.Minute)
		c.scan(time.Now())
		require.Equal(t, float64(2), counterVal(t, counters.rebuilds))
	})
}

func TestTTLRelabelCache_RebuildSkippedBelowFloor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		counters := newTestTTLCounters()
		c := newTTLRelabelCache(time.Minute, counters)
		defer c.Close()

		// Default floor is 4096 — far above the 100 entries we'll add.
		for i := uint64(0); i < 100; i++ {
			c.Add(i, labels.FromStrings("k", "v"))
		}
		time.Sleep(2 * time.Minute)
		c.scan(time.Now())

		require.Equal(t, 0, c.Len())
		require.Equal(t, float64(0), counterVal(t, counters.rebuilds), "below-floor caches skip rebuild")
	})
}
