package convert

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// staticAppendable always returns the same storage.Appender, mirroring
// appenderAppendable from the external test package.
type staticAppendable struct {
	Inner storage.Appender
}

var _ storage.Appendable = staticAppendable{}

func (sa staticAppendable) Appender(context.Context) storage.Appender {
	return sa.Inner
}

const gcTestPayload = `{
	"resource_metrics": [{
		"scope_metrics": [{
			"metrics": [{
				"name": "test_metric",
				"description": "A test gauge metric",
				"gauge": {
					"data_points": [{
						"time_unix_nano": 1000000000,
						"as_double": 1
					}]
				}
			}]
		}]
	}]
}`

func newGCTestConverter(t *testing.T) *Converter {
	t.Helper()

	var app testappender.Appender
	l := util.TestAlloyLogger(t)
	return New(l.Slog(), staticAppendable{Inner: &app}, Options{HonorMetadata: true})
}

func consumeGCTestPayload(t *testing.T, conv *Converter) {
	t.Helper()

	decoder := &pmetric.JSONUnmarshaler{}
	payload, err := decoder.UnmarshalMetrics([]byte(gcTestPayload))
	require.NoError(t, err)
	require.NoError(t, conv.ConsumeMetrics(t.Context(), payload))
}

func ageMetadataEntry(t *testing.T, conv *Converter, name string, age time.Duration) {
	t.Helper()

	val, ok := conv.metadataCache.Load(name)
	require.True(t, ok, "expected a metadata entry for %s", name)

	md := val.(*memoryMetadata)
	md.Lock()
	md.lastSeen = time.Now().Add(-age)
	md.Unlock()
}

func seriesCacheLen(conv *Converter) int {
	var count int
	conv.seriesCache.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// TestConverterGCEvictsStaleMetadata pins that GC evicts stale entries from
// the metadata cache itself. The metadata entries are keyed by metric family
// name, so deleting the stale entry from the series cache (keyed by the full
// label set) was a no-op and stale metadata accumulated forever.
func TestConverterGCEvictsStaleMetadata(t *testing.T) {
	conv := newGCTestConverter(t)
	consumeGCTestPayload(t, conv)

	const staleTime = time.Hour

	// Age only the test_metric family metadata; target_info stays fresh.
	ageMetadataEntry(t, conv, "test_metric", 2*staleTime)

	seriesBefore := seriesCacheLen(conv)

	conv.GC(staleTime)

	_, ok := conv.metadataCache.Load("test_metric")
	require.False(t, ok, "GC must evict stale metadata entries from the metadata cache")

	_, ok = conv.metadataCache.Load("target_info")
	require.True(t, ok, "GC must retain fresh metadata entries")

	require.Equal(t, seriesBefore, seriesCacheLen(conv), "GC must not evict fresh series entries")
}

// TestConverterGCEvictsStaleSeries pins that GC continues to evict stale
// entries from the series cache.
func TestConverterGCEvictsStaleSeries(t *testing.T) {
	conv := newGCTestConverter(t)
	consumeGCTestPayload(t, conv)

	const staleTime = time.Hour

	conv.seriesCache.Range(func(_, value any) bool {
		series := value.(*memorySeries)
		series.Lock()
		series.lastSeen = time.Now().Add(-2 * staleTime)
		series.Unlock()
		return true
	})

	conv.GC(staleTime)

	require.Zero(t, seriesCacheLen(conv), "GC must evict stale series entries")
}

// TestConverterGCRetainsFreshEntries pins that GC does not over-evict: entries
// refreshed within the staleness window survive the sweep.
func TestConverterGCRetainsFreshEntries(t *testing.T) {
	conv := newGCTestConverter(t)
	consumeGCTestPayload(t, conv)

	seriesBefore := seriesCacheLen(conv)

	conv.GC(5 * time.Minute)

	_, ok := conv.metadataCache.Load("test_metric")
	require.True(t, ok, "GC must retain metadata refreshed within the staleness window")

	require.Equal(t, seriesBefore, seriesCacheLen(conv), "GC must retain series refreshed within the staleness window")
}
