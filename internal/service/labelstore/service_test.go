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
	l := labels.FromStrings("__name__", "test")
	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeSameGlobalID := mapping.GetOrAddGlobalRefID(l)
	require.True(t, globalID == shouldBeSameGlobalID)
	require.Len(t, mapping.labelsHashToGlobal, 1)
}

func TestAddingDifferentMarkers(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.FromStrings("__name__", "test")
	l2 := labels.FromStrings("__name__", "roar")
	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeDifferentID := mapping.GetOrAddGlobalRefID(l2)
	require.True(t, globalID != shouldBeDifferentID)
	require.Len(t, mapping.labelsHashToGlobal, 2)
}

func TestAddingLocalMapping(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	mapping.AddLocalLink("1", globalID, 1)
	mapping.GetLocalRefID("1", globalID)
	require.Equal(t, uint64(1), mapping.GetLocalRefID("1", globalID))
	require.Len(t, mapping.labelsHashToGlobal, 1)
	require.Len(t, mapping.mappings, 1)
	require.True(t, mapping.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.mappings["1"].globalToLocal[globalID] == 1)
	require.True(t, mapping.mappings["1"].localToGlobal[1] == globalID)
}

func TestAddingLocalMappings(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	localRefID := uint64(1)
	mapping.AddLocalLink("1", globalID, localRefID)
	mapping.AddLocalLink("2", globalID, localRefID)
	require.Equal(t, localRefID, mapping.GetLocalRefID("1", globalID))
	require.Equal(t, localRefID, mapping.GetLocalRefID("2", globalID))
	require.Len(t, mapping.labelsHashToGlobal, 1)
	require.Len(t, mapping.mappings, 2)

	require.True(t, mapping.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.mappings["1"].globalToLocal[globalID] == localRefID)
	require.True(t, mapping.mappings["1"].localToGlobal[localRefID] == globalID)

	require.True(t, mapping.mappings["2"].RemoteWriteID == "2")
	require.True(t, mapping.mappings["2"].globalToLocal[globalID] == localRefID)
	require.True(t, mapping.mappings["2"].localToGlobal[localRefID] == globalID)
}

func TestReplaceLocalMappings(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	localRefID := uint64(1)
	mapping.AddLocalLink("1", globalID, localRefID)
	mapping.AddLocalLink("2", globalID, localRefID)
	require.Equal(t, localRefID, mapping.GetLocalRefID("1", globalID))
	require.Equal(t, localRefID, mapping.GetLocalRefID("2", globalID))

	localRefID = uint64(2)
	mapping.ReplaceLocalLink("1", globalID, 1, localRefID)
	mapping.ReplaceLocalLink("2", globalID, 1, localRefID)
	require.Len(t, mapping.labelsHashToGlobal, 1)
	require.Len(t, mapping.mappings, 2)

	require.True(t, mapping.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.mappings["1"].globalToLocal[globalID] == localRefID)
	require.Len(t, mapping.mappings["1"].localToGlobal, 1)
	require.True(t, mapping.mappings["1"].localToGlobal[localRefID] == globalID)

	require.True(t, mapping.mappings["2"].RemoteWriteID == "2")
	require.True(t, mapping.mappings["2"].globalToLocal[globalID] == localRefID)
	require.Len(t, mapping.mappings["2"].localToGlobal, 1)
	require.True(t, mapping.mappings["2"].localToGlobal[localRefID] == globalID)
}

func TestReplaceWithoutAddingLocalMapping(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	localRefID := uint64(2)
	mapping.ReplaceLocalLink("1", globalID, 1, localRefID)
	mapping.ReplaceLocalLink("2", globalID, 1, localRefID)

	require.Equal(t, localRefID, mapping.GetLocalRefID("1", globalID))
	require.Equal(t, localRefID, mapping.GetLocalRefID("2", globalID))
}

func TestStaleness(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer)
	l := labels.FromStrings("__name__", "test")
	l2 := labels.FromStrings("__name__", "test2")

	global1 := mapping.GetOrAddGlobalRefID(l)
	global2 := mapping.GetOrAddGlobalRefID(l2)
	mapping.AddLocalLink("1", global1, 1)
	mapping.AddLocalLink("2", global2, 1)
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
	l := labels.FromStrings("__name__", "test")

	global1 := mapping.GetOrAddGlobalRefID(l)
	mapping.AddLocalLink("1", global1, 1)
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
	for i := range 100_000 {
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
		wg.Go(func() {
			ls.TrackStaleness(tracking)
		})
	}
	wg.Wait()
}
