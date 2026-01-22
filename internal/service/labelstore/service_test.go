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
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
	l := labels.FromStrings("__name__", "test")
	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeSameGlobalID := mapping.GetOrAddGlobalRefID(l)
	require.True(t, globalID == shouldBeSameGlobalID)
	require.Len(t, mapping.single.labelsHashToGlobal, 1)
}

func TestAddingDifferentMarkers(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
	l := labels.FromStrings("__name__", "test")
	l2 := labels.FromStrings("__name__", "roar")
	globalID := mapping.GetOrAddGlobalRefID(l)
	shouldBeDifferentID := mapping.GetOrAddGlobalRefID(l2)
	require.True(t, globalID != shouldBeDifferentID)
	require.Len(t, mapping.single.labelsHashToGlobal, 2)
}

func TestAddingLocalMapping(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	mapping.AddLocalLink("1", globalID, 1)
	mapping.GetLocalRefID("1", globalID)
	require.Equal(t, uint64(1), mapping.GetLocalRefID("1", globalID))
	require.Len(t, mapping.single.labelsHashToGlobal, 1)
	require.Len(t, mapping.single.mappings, 1)
	require.True(t, mapping.single.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.single.mappings["1"].globalToLocal[globalID] == 1)
	require.True(t, mapping.single.mappings["1"].localToGlobal[1] == globalID)
}

func TestAddingLocalMappings(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	localRefID := uint64(1)
	mapping.AddLocalLink("1", globalID, localRefID)
	mapping.AddLocalLink("2", globalID, localRefID)
	require.Equal(t, localRefID, mapping.GetLocalRefID("1", globalID))
	require.Equal(t, localRefID, mapping.GetLocalRefID("2", globalID))
	require.Len(t, mapping.single.labelsHashToGlobal, 1)
	require.Len(t, mapping.single.mappings, 2)

	require.True(t, mapping.single.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.single.mappings["1"].globalToLocal[globalID] == localRefID)
	require.True(t, mapping.single.mappings["1"].localToGlobal[localRefID] == globalID)

	require.True(t, mapping.single.mappings["2"].RemoteWriteID == "2")
	require.True(t, mapping.single.mappings["2"].globalToLocal[globalID] == localRefID)
	require.True(t, mapping.single.mappings["2"].localToGlobal[localRefID] == globalID)
}

func TestReplaceLocalMappings(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
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
	require.Len(t, mapping.single.labelsHashToGlobal, 1)
	require.Len(t, mapping.single.mappings, 2)

	require.True(t, mapping.single.mappings["1"].RemoteWriteID == "1")
	require.True(t, mapping.single.mappings["1"].globalToLocal[globalID] == localRefID)
	require.Len(t, mapping.single.mappings["1"].localToGlobal, 1)
	require.True(t, mapping.single.mappings["1"].localToGlobal[localRefID] == globalID)

	require.True(t, mapping.single.mappings["2"].RemoteWriteID == "2")
	require.True(t, mapping.single.mappings["2"].globalToLocal[globalID] == localRefID)
	require.Len(t, mapping.single.mappings["2"].localToGlobal, 1)
	require.True(t, mapping.single.mappings["2"].localToGlobal[localRefID] == globalID)
}

func TestReplaceWithoutAddingLocalMapping(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
	l := labels.FromStrings("__name__", "test")

	globalID := mapping.GetOrAddGlobalRefID(l)
	localRefID := uint64(2)
	mapping.ReplaceLocalLink("1", globalID, 1, localRefID)
	mapping.ReplaceLocalLink("2", globalID, 1, localRefID)

	require.Equal(t, localRefID, mapping.GetLocalRefID("1", globalID))
	require.Equal(t, localRefID, mapping.GetLocalRefID("2", globalID))
}

func TestStaleness(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
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
	require.Len(t, mapping.single.staleGlobals, 1)
	require.Len(t, mapping.single.labelsHashToGlobal, 2)
	staleDuration = 1 * time.Millisecond
	time.Sleep(10 * time.Millisecond)
	mapping.CheckAndRemoveStaleMarkers()
	require.Len(t, mapping.single.staleGlobals, 0)
	require.Len(t, mapping.single.labelsHashToGlobal, 1)
}

func TestRemovingStaleness(t *testing.T) {
	mapping := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)
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

	require.Len(t, mapping.single.staleGlobals, 1)
	// This should remove it from staleness tracking.
	mapping.TrackStaleness([]StalenessTracker{
		{
			GlobalRefID: global1,
			Value:       1,
			Labels:      l,
		},
	})
	require.Len(t, mapping.single.staleGlobals, 0)
}

// TestLabelStoreBasicOperations tests basic labelstore operations without accessing internal state.
// This test can run with any shard configuration.
func TestLabelStoreBasicOperations(t *testing.T) {
	testCases := []struct {
		name   string
		shards int
	}{
		{"single shard", 1},
		{"4 shards", 4},
		{"32 shards", 32},
		{"64 shards", 64},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ls := New(log.NewNopLogger(), prometheus.NewRegistry(), tc.shards)

			// Test: Same labels should return same global ID
			l1 := labels.FromStrings("__name__", "http_requests", "path", "/api")
			globalID1 := ls.GetOrAddGlobalRefID(l1)
			globalID2 := ls.GetOrAddGlobalRefID(l1)
			require.Equal(t, globalID1, globalID2, "same labels should return same global ID")
			require.NotZero(t, globalID1, "global ID should not be zero")

			// Test: Different labels should return different IDs
			l2 := labels.FromStrings("__name__", "http_requests", "path", "/metrics")
			globalID3 := ls.GetOrAddGlobalRefID(l2)
			require.NotEqual(t, globalID1, globalID3, "different labels should return different IDs")

			// Test: Empty labels should return 0
			empty := labels.Labels{}
			require.Zero(t, ls.GetOrAddGlobalRefID(empty), "empty labels should return 0")
		})
	}
}

// TestLabelStoreLocalMapping tests local ID mapping without accessing internal state.
func TestLabelStoreLocalMapping(t *testing.T) {
	testCases := []struct {
		name   string
		shards int
	}{
		{"single shard", 1},
		{"4 shards", 4},
		{"32 shards", 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ls := New(log.NewNopLogger(), prometheus.NewRegistry(), tc.shards)

			l := labels.FromStrings("__name__", "test", "job", "api")
			globalID := ls.GetOrAddGlobalRefID(l)

			// Test: Initially no local mapping exists
			localID := ls.GetLocalRefID("component1", globalID)
			require.Zero(t, localID, "should return 0 when no mapping exists")

			// Test: Add local mapping
			ls.AddLocalLink("component1", globalID, 100)
			localID = ls.GetLocalRefID("component1", globalID)
			require.Equal(t, uint64(100), localID, "should return added local ID")

			// Test: Different components can have different local IDs for same global ID
			ls.AddLocalLink("component2", globalID, 200)
			require.Equal(t, uint64(100), ls.GetLocalRefID("component1", globalID))
			require.Equal(t, uint64(200), ls.GetLocalRefID("component2", globalID))
		})
	}
}

// TestLabelStoreReplaceMapping tests replacing local mappings without accessing internal state.
func TestLabelStoreReplaceMapping(t *testing.T) {
	testCases := []struct {
		name   string
		shards int
	}{
		{"single shard", 1},
		{"4 shards", 4},
		{"32 shards", 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ls := New(log.NewNopLogger(), prometheus.NewRegistry(), tc.shards)

			l := labels.FromStrings("__name__", "test")
			globalID := ls.GetOrAddGlobalRefID(l)

			// Add initial mapping
			ls.AddLocalLink("component1", globalID, 100)
			require.Equal(t, uint64(100), ls.GetLocalRefID("component1", globalID))

			// Replace with new local ID
			ls.ReplaceLocalLink("component1", globalID, 100, 200)
			require.Equal(t, uint64(200), ls.GetLocalRefID("component1", globalID))

			// Replace on non-existent component should create new mapping
			ls.ReplaceLocalLink("component2", globalID, 0, 300)
			require.Equal(t, uint64(300), ls.GetLocalRefID("component2", globalID))
		})
	}
}

// TestLabelStoreConcurrent tests concurrent access patterns without accessing internal state.
func TestLabelStoreConcurrent(t *testing.T) {
	testCases := []struct {
		name   string
		shards int
	}{
		{"single shard", 1},
		{"4 shards", 4},
		{"32 shards", 32},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ls := New(log.NewNopLogger(), prometheus.NewRegistry(), tc.shards)

			const numGoroutines = 100
			const numLabels = 50

			// Pre-generate labels
			labelSets := make([]labels.Labels, numLabels)
			for i := 0; i < numLabels; i++ {
				labelSets[i] = labels.FromStrings(
					"__name__", "metric",
					"id", strconv.Itoa(i),
				)
			}

			// Phase 1: Pre-populate global IDs to avoid race on slice writes
			globalIDs := make([]uint64, numLabels)
			for i := 0; i < numLabels; i++ {
				globalIDs[i] = ls.GetOrAddGlobalRefID(labelSets[i])
			}

			// Phase 2: Concurrent reads - verify same labels return same ID
			var wg sync.WaitGroup
			wg.Add(numGoroutines)
			for g := 0; g < numGoroutines; g++ {
				go func(idx int) {
					defer wg.Done()
					labelIdx := idx % numLabels
					id := ls.GetOrAddGlobalRefID(labelSets[labelIdx])
					// All goroutines accessing same labels should get same ID
					require.Equal(t, globalIDs[labelIdx], id)
				}(g)
			}
			wg.Wait()

			// Phase 3: Concurrent local mapping operations
			// Each goroutine uses a unique component ID to avoid conflicts
			wg.Add(numGoroutines)
			for g := 0; g < numGoroutines; g++ {
				go func(idx int) {
					defer wg.Done()
					labelIdx := idx % numLabels
					componentID := "component_" + strconv.Itoa(idx) // Unique per goroutine
					localID := uint64(idx * 100)

					ls.AddLocalLink(componentID, globalIDs[labelIdx], localID)
					result := ls.GetLocalRefID(componentID, globalIDs[labelIdx])
					require.Equal(t, localID, result, "should get back the local ID we just added")
				}(g)
			}
			wg.Wait()

			// Phase 4: Verify mappings persist (sequential, no race)
			for g := 0; g < numGoroutines; g++ {
				labelIdx := g % numLabels
				componentID := "component_" + strconv.Itoa(g)
				expectedLocal := uint64(g * 100)

				result := ls.GetLocalRefID(componentID, globalIDs[labelIdx])
				require.Equal(t, expectedLocal, result, "mapping should persist")
			}
		})
	}
}

func BenchmarkStaleness(b *testing.B) {
	b.StopTimer()
	ls := New(log.NewNopLogger(), prometheus.DefaultRegisterer, 1)

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

// BenchmarkHighContention simulates the real prometheus write path roughly matching production traffic.
func BenchmarkHighContention(b *testing.B) {
	const numGoroutines = 20000
	const numUniqueLabelSets = 10000
	const numComponents = 5
	const numShards = 32

	ls := New(log.NewNopLogger(), prometheus.NewRegistry(), numShards)

	// Pre-generate label sets
	labelSets := make([]labels.Labels, numUniqueLabelSets)
	for i := range numUniqueLabelSets {
		labelSets[i] = labels.FromStrings(
			"__name__", "http_requests_total",
			"job", "api-server-"+strconv.Itoa(i),
			"instance", "10.0.0."+strconv.Itoa(i%256),
			"path", "/api/v1/query_"+strconv.Itoa(i%100),
			"method", []string{"GET", "POST", "PUT", "DELETE"}[i%4],
			"status", strconv.Itoa(200+i%100),
		)
	}

	// Pre-populate 50% of series to simulate existing series (warm cache)
	// This means 50% of GetOrAddGlobalRefID calls will be cache hits, 50% will create new entries
	for i := range numUniqueLabelSets / 2 {
		globalID := ls.GetOrAddGlobalRefID(labelSets[i])
		for c := range numComponents {
			componentID := "component_" + strconv.Itoa(c)
			ls.AddLocalLink(componentID, globalID, uint64(i*100+c))
		}
	}

	for i := 0; b.Loop(); i++ {
		var wg sync.WaitGroup

		// Simulate prometheus metrics collection happening in background
		if i%10 == 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Simulate prometheus calling Collect
				ch := make(chan prometheus.Metric, 10)
				go func() {
					for range ch {
					}
				}()
				ls.Collect(ch)
				close(ch)
			}()
		}

		for g := range numGoroutines {
			wg.Add(1)
			go func(gIdx int) {
				defer wg.Done()
				lblIdx := (gIdx * 7) % numUniqueLabelSets
				componentID := "component_" + strconv.Itoa(gIdx%numComponents)

				// This is called on EVERY sample in production to get the global ref ID.
				globalRef := ls.GetOrAddGlobalRefID(labelSets[lblIdx])

				// Read-only operation to get the cached local ref for this component.
				cachedLocalRef := ls.GetLocalRefID(componentID, globalRef)

				// Simulate getting a new local ref from storage.Append
				newLocalRef := uint64(lblIdx*1000 + gIdx)

				if cachedLocalRef == 0 {
					// First time seeing this series for this component
					ls.AddLocalLink(componentID, globalRef, newLocalRef)
				} else {
					// Local ref changed, need to replace
					ls.ReplaceLocalLink(componentID, globalRef, cachedLocalRef, newLocalRef)
				}

				// Occasionally track staleness (~10% of samples)
				if gIdx%10 == 0 {
					ls.TrackStaleness([]StalenessTracker{{
						GlobalRefID: globalRef,
						Value:       math.Float64frombits(value.StaleNaN),
						Labels:      labelSets[lblIdx],
					}})
				}
			}(g)
		}

		wg.Wait()
	}
}
