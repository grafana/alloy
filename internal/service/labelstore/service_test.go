package labelstore

import (
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/stretchr/testify/require"
)

func TestAddingMarker(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})
	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeSameGlobalID := mapping.GetOrAddGlobalRefID(l)
	require.True(t, globalID == shouldBeSameGlobalID)
	require.Len(t, mapping.labelsHashToGlobal, 1)
}

func TestAddingDifferentMarkers(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})
	l2 := labels.Labels{}
	l2 = append(l2, labels.Label{
		Name:  "__name__",
		Value: "roar",
	})
	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeDifferentID := mapping.GetOrAddGlobalRefID(l2)
	require.True(t, globalID != shouldBeDifferentID)
	require.Len(t, mapping.labelsHashToGlobal, 2)
}

func TestAddingLocalMapping(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})

	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeSameGlobalID := mapping.GetOrAddLink("1", 1, l)
	require.True(t, globalID == shouldBeSameGlobalID)
	require.Len(t, mapping.labelsHashToGlobal, 1)
	require.Len(t, mapping.mappings, 1)
	require.True(t, mapping.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.mappings["1"].globalToLocal[shouldBeSameGlobalID] == 1)
	require.True(t, mapping.mappings["1"].localToGlobal[1] == shouldBeSameGlobalID)
}

func TestAddingLocalMappings(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})

	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeSameGlobalID := mapping.GetOrAddLink("1", 1, l)
	shouldBeSameGlobalID2 := mapping.GetOrAddLink("2", 1, l)
	require.True(t, globalID == shouldBeSameGlobalID)
	require.True(t, globalID == shouldBeSameGlobalID2)
	require.Len(t, mapping.labelsHashToGlobal, 1)
	require.Len(t, mapping.mappings, 2)

	require.True(t, mapping.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.mappings["1"].globalToLocal[shouldBeSameGlobalID] == 1)
	require.True(t, mapping.mappings["1"].localToGlobal[1] == shouldBeSameGlobalID)

	require.True(t, mapping.mappings["2"].RemoteWriteID == "2")
	require.True(t, mapping.mappings["2"].globalToLocal[shouldBeSameGlobalID2] == 1)
	require.True(t, mapping.mappings["2"].localToGlobal[1] == shouldBeSameGlobalID2)
}

func TestAddingLocalMappingsWithoutCreatingGlobalUpfront(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})

	shouldBeSameGlobalID := mapping.GetOrAddLink("1", 1, l)
	shouldBeSameGlobalID2 := mapping.GetOrAddLink("2", 1, l)
	require.True(t, shouldBeSameGlobalID2 == shouldBeSameGlobalID)
	require.Len(t, mapping.labelsHashToGlobal, 1)
	require.Len(t, mapping.mappings, 2)

	require.True(t, mapping.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.mappings["1"].globalToLocal[shouldBeSameGlobalID] == 1)
	require.True(t, mapping.mappings["1"].localToGlobal[1] == shouldBeSameGlobalID)

	require.True(t, mapping.mappings["2"].RemoteWriteID == "2")
	require.True(t, mapping.mappings["2"].globalToLocal[shouldBeSameGlobalID2] == 1)
	require.True(t, mapping.mappings["2"].localToGlobal[1] == shouldBeSameGlobalID2)
}

func TestStaleness(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})
	l2 := labels.Labels{}
	l2 = append(l2, labels.Label{
		Name:  "__name__",
		Value: "test2",
	})

	global1 := mapping.GetOrAddLink("1", 1, l)
	_ = mapping.GetOrAddLink("2", 1, l2)
	mapping.TrackStaleness([]StalenessTracker{
		{
			GlobalRefID: global1,
			Value:       math.Float64frombits(value.StaleNaN),
			Labels:      l,
		},
	})
	require.Len(t, mapping.staleGlobals, 1)
	require.Len(t, mapping.labelsHashToGlobal, 2)
	staleDuration = 1 * time.Millisecond
	time.Sleep(10 * time.Millisecond)
	mapping.CheckAndRemoveStaleMarkers()
	require.Len(t, mapping.staleGlobals, 0)
	require.Len(t, mapping.labelsHashToGlobal, 1)
}

func TestRemovingStaleness(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.Labels{}
	l = append(l, labels.Label{
		Name:  "__name__",
		Value: "test",
	})

	global1 := mapping.GetOrAddLink("1", 1, l)
	mapping.TrackStaleness([]StalenessTracker{
		{
			GlobalRefID: global1,
			Value:       math.Float64frombits(value.StaleNaN),
			Labels:      l,
		},
	})

	require.Len(t, mapping.staleGlobals, 1)
	// This should remove it from staleness tracking.
	mapping.TrackStaleness([]StalenessTracker{
		{
			GlobalRefID: global1,
			Value:       1,
			Labels:      l,
		},
	})
	require.Len(t, mapping.staleGlobals, 0)
}

func BenchmarkStaleness(b *testing.B) {
	b.StopTimer()
	ls := New(log.NewNopLogger(), prometheus.DefaultRegisterer)

	tracking := make([]StalenessTracker, 100_000)
	for i := 0; i < 100_000; i++ {
		l := labels.FromStrings("id", strconv.Itoa(i))
		gid := ls.GetOrAddGlobalRefID(l)
		var val float64
		if i%2 == 0 {
			val = float64(i)
		} else {
			val = math.Float64frombits(value.StaleNaN)
		}
		tracking[i] = StalenessTracker{
			GlobalRefID: gid,
			Value:       val,
			Labels:      l,
		}
	}
	b.StartTimer()
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			ls.TrackStaleness(tracking)
			wg.Done()
		}()
	}
	wg.Wait()
}
