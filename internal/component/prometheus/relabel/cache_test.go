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

func TestTTLRelabelCache_LazyExpiry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		raw, err := newRelabelCache(Arguments{CacheTTL: 30 * time.Second}, newTestEvictionsCounter())
		require.NoError(t, err)
		c := raw.(*ttlRelabelCache)
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
