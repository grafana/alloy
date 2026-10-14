package appenders

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestSeriesRefMappingStore_GetMappingReturnsNilForUnknownRef(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	require.Nil(t, store.GetMapping(0, labels.EmptyLabels(), 0))
	require.Nil(t, store.GetMapping(1, labels.EmptyLabels(), 0))
	require.Nil(t, store.GetMapping(999, labels.EmptyLabels(), 0))
	require.Nil(t, store.GetMapping(storage.SeriesRef(12345), labels.EmptyLabels(), 0))
}

func TestSeriesRefMappingStore_CreatedMappingCanBeRetrieved(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	childRefs := []storage.SeriesRef{1, 2, 3}
	lbls := labels.NewBuilder(labels.EmptyLabels()).Set("foo", "bar").Labels()

	uniqueRef := store.CreateMapping(childRefs, lbls, 0)

	// Case 1: get by unique ref
	got := store.GetMapping(uniqueRef, lbls, 0)
	require.NotNil(t, got)
	require.Equal(t, childRefs, got)

	// Case 1: rely on label hash fallback
	got = store.GetMapping(0, lbls, 0)
	require.NotNil(t, got)
	require.Equal(t, childRefs, got)
}

func TestSeriesRefMappingStore_EachCreatedMappingGetsUniqueRef(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	type mappingAndLabels struct {
		refs   []storage.SeriesRef
		labels labels.Labels
	}

	refs := make(map[storage.SeriesRef]bool)
	mappings := make(map[storage.SeriesRef]mappingAndLabels)

	for i := range 100 {
		lbls := labels.NewBuilder(labels.EmptyLabels()).Set("k", strconv.Itoa(i)).Labels()
		childRefs := []storage.SeriesRef{storage.SeriesRef(i), storage.SeriesRef(i + 1)}
		uniqueRef := store.CreateMapping(childRefs, lbls, 0)

		// Verify this ref is unique
		require.False(t, refs[uniqueRef], "ref %d was already used", uniqueRef)
		refs[uniqueRef] = true
		mappings[uniqueRef] = mappingAndLabels{
			refs:   childRefs,
			labels: lbls,
		}
	}

	// Verify all mappings can be retrieved independently
	for uniqueRef, mp := range mappings {
		retrieved := store.GetMapping(uniqueRef, mp.labels, 0)
		require.Equal(t, mp.refs, retrieved)
	}
}

func TestSeriesRefMappingStore_UpdateMappingChangesReturnedValue(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	originalRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(originalRefs, lbls, 0)

	updatedRefs := []storage.SeriesRef{4, 5, 6}
	store.UpdateMapping(uniqueRef, updatedRefs, lbls, 0)

	retrieved := store.GetMapping(uniqueRef, lbls, 0)
	require.Equal(t, updatedRefs, retrieved)
	require.NotEqual(t, originalRefs, retrieved)
}

func TestSeriesRefMappingStore_UpdateMappingWithZeroRefDoesNothing(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	lbls := labels.EmptyLabels()

	store.UpdateMapping(0, []storage.SeriesRef{1, 2, 3}, lbls, 0)

	// Should still return nil
	require.Nil(t, store.GetMapping(0, lbls, 0))
}

func TestSeriesRefMappingStore_UpdateMappingAfterRemovalDoesNotRecreate(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.FromStrings("k", "v")

	uniqueRef := store.CreateMapping([]storage.SeriesRef{1}, lbls, 0)
	trackRef(store, uniqueRef, time.Now().Unix()-3600)
	store.removeStaleRefs(time.Now().Unix())

	store.UpdateMapping(uniqueRef, []storage.SeriesRef{2}, lbls, 0)

	require.Empty(t, store.uniqueRefToChildRefs)
	require.Empty(t, store.labelHashToUniqueRef)
	require.Equal(t, 0.0, testutil.ToFloat64(store.activeMappings))
}

func TestSeriesRefMappingStore_TrackAppendedSeriesDoesNotPanic(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	cell := store.GetCellForAppendedSeries()
	cell.Refs = append(cell.Refs, 1, 2, 3)

	require.NotPanics(t, func() {
		store.TrackAppendedSeries(time.Now().Unix(), cell)
	})
}

func TestSeriesRefMappingStore_SliceIsEmptyAfterReturn(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	cell1 := store.GetCellForAppendedSeries()
	cell1.Refs = append(cell1.Refs, 1, 2, 3)
	store.TrackAppendedSeries(time.Now().Unix(), cell1)

	cell2 := store.GetCellForAppendedSeries()
	require.NotNil(t, cell2)
	require.Equal(t, 0, len(cell2.Refs), "slice returned should always have length 0")
}

func TestSeriesRefMappingStore_RefsAreEventuallyCleanedUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Stop)
		lbls := labels.EmptyLabels()

		// Create and track a mapping with old timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs, lbls, 0)

		oldTimestamp := time.Now().Add(-20 * time.Minute).Unix()
		cell := store.GetCellForAppendedSeries()
		cell.Refs = append(cell.Refs, uniqueRef)
		store.TrackAppendedSeries(oldTimestamp, cell)

		// Verify mapping exists initially
		require.NotNil(t, store.GetMapping(uniqueRef, lbls, 0))

		// Wait for cleanup to run (15 minute ticker + some buffer)
		time.Sleep(16 * time.Minute)

		// Mapping should be cleaned up
		require.Nil(t, store.GetMapping(uniqueRef, lbls, 0))
	})
}

func TestSeriesRefMappingStore_RecentlyTrackedRefsAreNotCleanedUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Stop)
		lbls := labels.EmptyLabels()

		// Create and track a mapping with recent timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs, lbls, 0)

		recentTimestamp := time.Now().Unix()
		cell := store.GetCellForAppendedSeries()
		cell.Refs = append(cell.Refs, uniqueRef)
		store.TrackAppendedSeries(recentTimestamp, cell)

		// Wait for a cleanup cycle
		time.Sleep(16 * time.Minute)

		// Mapping should still exist
		require.NotNil(t, store.GetMapping(uniqueRef, lbls, 0))
	})
}

func TestSeriesRefMappingStore_TrackingRefAgainUpdatesTimestamp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Stop)
		lbls := labels.EmptyLabels()

		// Create and track a mapping with old timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs, lbls, 0)

		oldTimestamp := time.Now().Add(-20 * time.Minute).Unix()
		cell1 := store.GetCellForAppendedSeries()
		cell1.Refs = append(cell1.Refs, uniqueRef)
		store.TrackAppendedSeries(oldTimestamp, cell1)

		// Wait a bit
		time.Sleep(1 * time.Minute)

		// Track the same ref again with current timestamp
		currentTimestamp := time.Now().Unix()
		cell2 := store.GetCellForAppendedSeries()
		cell2.Refs = append(cell2.Refs, uniqueRef)
		store.TrackAppendedSeries(currentTimestamp, cell2)

		// Wait for cleanup cycle
		time.Sleep(16 * time.Minute)

		// Mapping should NOT be cleaned up because timestamp was refreshed
		require.NotNil(t, store.GetMapping(uniqueRef, lbls, 0))
	})
}

func TestSeriesRefMappingStore_RemoveStaleRefs(t *testing.T) {
	tests := []struct {
		name  string
		fresh int
		stale int
		// orphans are refs tracked with no mapping behind them, as happens when a batch
		// commits after cleanup has already evicted the ref it read.
		orphans int
	}{
		{name: "removes stale and keeps fresh", fresh: 1, stale: 1},
		{name: "nothing stale", fresh: 1},
		{name: "exactly one chunk", stale: staleRefDeleteChunk},
		{name: "across a chunk boundary", stale: staleRefDeleteChunk + 100},
		{name: "orphan timestamps do not skew the gauge", fresh: 1, stale: 1, orphans: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewSeriesRefMappingStore(nil)
			t.Cleanup(store.Stop)

			now := time.Now().Unix()
			create := func(prefix string, n int, ts int64) ([]storage.SeriesRef, []labels.Labels) {
				refs := make([]storage.SeriesRef, n)
				lbls := make([]labels.Labels, n)
				for i := range n {
					lbls[i] = labels.FromStrings("k", prefix+strconv.Itoa(i))
					refs[i] = store.CreateMapping([]storage.SeriesRef{storage.SeriesRef(i + 1)}, lbls[i], 0)
					trackRef(store, refs[i], ts)
				}
				return refs, lbls
			}
			freshRefs, freshLbls := create("fresh", tt.fresh, now)
			staleRefs, staleLbls := create("stale", tt.stale, now-3600)
			for i := range tt.orphans {
				trackRef(store, storage.SeriesRef(1_000_000+i), now-3600)
			}

			store.removeStaleRefs(now - 60)

			for i, ref := range staleRefs {
				require.Nil(t, store.GetMapping(ref, staleLbls[i], 0), "stale mapping should be removed")
				require.NotContains(t, store.uniqueRefTimestamps, ref)
			}
			for i, ref := range freshRefs {
				require.NotNil(t, store.GetMapping(ref, freshLbls[i], 0), "fresh mapping should survive")
				require.Contains(t, store.uniqueRefTimestamps, ref)
			}
			require.Len(t, store.uniqueRefToChildRefs, tt.fresh)
			require.Len(t, store.labelHashToUniqueRef, tt.fresh)
			require.Len(t, store.uniqueRefTimestamps, tt.fresh)

			require.Equal(t, float64(tt.fresh), testutil.ToFloat64(store.activeMappings))
			require.Equal(t, float64(tt.fresh), testutil.ToFloat64(store.trackedRefs))
			require.Equal(t, float64(tt.stale+tt.orphans), testutil.ToFloat64(store.refsCleaned))
		})
	}
}

func TestSeriesRefMappingStore_RemoveStaleRefsRacesWithAppends(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	const series = 128
	lbls := make([]labels.Labels, series)
	refs := make([]storage.SeriesRef, series)
	for i := range series {
		lbls[i] = labels.FromStrings("k", strconv.Itoa(i))
		refs[i] = store.CreateMapping([]storage.SeriesRef{storage.SeriesRef(i + 1)}, lbls[i], 0)
	}

	stop := make(chan struct{})
	var wg, ready sync.WaitGroup
	ready.Add(8)
	for w := range 8 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			first := true
			for i := w; ; i = (i + 8) % series {
				select {
				case <-stop:
					return
				default:
				}
				store.GetMapping(refs[i], lbls[i], 0)
				trackRef(store, refs[i], time.Now().Unix())
				if first {
					ready.Done()
					first = false
				}
			}
		}(w)
	}

	// Wait for every worker to be inside its loop, otherwise the passes below can finish
	// before the workers are scheduled and nothing actually overlaps.
	ready.Wait()

	// A cutoff in the future makes every tracked ref stale, so each pass deletes whatever
	// the workers just tracked.
	for range 200 {
		store.removeStaleRefs(time.Now().Unix() + 60)
	}
	close(stop)
	wg.Wait()

	// Whatever survived must be mutually consistent: each reverse entry resolves to a
	// live mapping carrying the same hash.
	for hash, ref := range store.labelHashToUniqueRef {
		mapping, ok := store.uniqueRefToChildRefs[ref]
		require.True(t, ok, "labelHashToUniqueRef points at a deleted mapping")
		require.Equal(t, hash, mapping.labelHash)
	}

	// The store is still usable afterward.
	newLbls := labels.FromStrings("k", "after")
	newRef := store.CreateMapping([]storage.SeriesRef{42}, newLbls, 0)
	require.Equal(t, []storage.SeriesRef{42}, store.GetMapping(newRef, newLbls, 0))
}

// trackRef records ts as ref's last-append time via the normal tracking path.
func trackRef(store *SeriesRefMappingStore, ref storage.SeriesRef, ts int64) {
	cell := store.GetCellForAppendedSeries()
	cell.Refs = append(cell.Refs, ref)
	store.TrackAppendedSeries(ts, cell)
}

func TestSeriesRefMappingStore_ClearRemovesAllMappings(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	// Create several mappings
	var uniqueRefs []storage.SeriesRef
	for i := range 10 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i), storage.SeriesRef(i + 1)}
		uniqueRef := store.CreateMapping(childRefs, lbls, 0)
		uniqueRefs = append(uniqueRefs, uniqueRef)

		// Track them
		cell := store.GetCellForAppendedSeries()
		cell.Refs = append(cell.Refs, uniqueRef)
		store.TrackAppendedSeries(time.Now().Unix(), cell)
	}

	// Verify they exist
	for _, ref := range uniqueRefs {
		require.NotNil(t, store.GetMapping(ref, lbls, 0))
	}

	// Clear advances the generation boundary past all previously issued refs.
	threshold := store.Clear()
	require.Greater(t, uint64(threshold), uint64(uniqueRefs[len(uniqueRefs)-1]),
		"threshold must be above all previously issued refs")

	// Verify all are gone
	for _, ref := range uniqueRefs {
		require.Nil(t, store.GetMapping(ref, lbls, store.Generation()))
	}
}

func TestSeriesRefMappingStore_ClearIsIdempotent(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	// Create some mappings
	childRefs := []storage.SeriesRef{1, 2, 3}
	store.CreateMapping(childRefs, lbls, 0)

	// Clear multiple times
	require.NotPanics(t, func() {
		store.Clear()
		store.Clear()
		store.Clear()
	})
}

func TestSeriesRefMappingStore_CanBeReusedAfterClear(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	// Create multiple mappings before clear
	childRefs1 := []storage.SeriesRef{1, 2, 3}
	uniqueRef1 := store.CreateMapping(childRefs1, lbls, 0)
	childRefs2 := []storage.SeriesRef{7, 8, 9}
	uniqueRef2 := store.CreateMapping(childRefs2, lbls, 0)

	// Clear
	store.Clear()
	gen := store.Generation()

	// Create new mappings after clear
	childRefs3 := []storage.SeriesRef{4, 5, 6}
	uniqueRef3 := store.CreateMapping(childRefs3, lbls, gen)

	// New mapping should work
	retrieved := store.GetMapping(uniqueRef3, lbls, gen)
	require.NotNil(t, retrieved)
	require.Equal(t, childRefs3, retrieved)

	// Old mappings should not exist
	require.Nil(t, store.GetMapping(uniqueRef1, lbls, gen))
	require.Nil(t, store.GetMapping(uniqueRef2, lbls, gen))
}

func TestSeriesRefMappingStore_ClearIsSafeWithConcurrentCellUse(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				// Get is the unlocked read Clear used to race with.
				store.GetCellForAppendedSeries()
			}
		})
	}

	for range 100_000 {
		store.Clear()
	}
	close(stop)
	wg.Wait()
}

func TestSeriesRefMappingStore_ClearIsSafeWithConcurrentCreates(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Go(func() {
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				// CreateMapping is what starts the cleanup goroutine.
				store.CreateMapping([]storage.SeriesRef{1}, labels.FromStrings("w", strconv.Itoa(w), "i", strconv.Itoa(i)), 0)
			}
		})
	}

	for range 1000 {
		store.Clear()
	}
	close(stop)
	wg.Wait()
}

func TestSeriesRefMappingStore_CleanupKeepsRunningAfterClear(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Stop)
		lbls := labels.FromStrings("k", "v")

		trackRef(store, store.CreateMapping([]storage.SeriesRef{1}, lbls, 0), time.Now().Unix())
		store.Clear()
		gen := store.Generation()

		uniqueRef := store.CreateMapping([]storage.SeriesRef{2}, lbls, gen)
		require.NotNil(t, store.GetMapping(uniqueRef, lbls, gen))
		trackRef(store, uniqueRef, time.Now().Add(-20*time.Minute).Unix())

		time.Sleep(16 * time.Minute)

		require.Nil(t, store.GetMapping(uniqueRef, lbls, gen))
	})
}

func TestSeriesRefMappingStore_IgnoresStaleGeneration(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.FromStrings("k", "v")

	stale := store.Generation()
	store.Clear()
	gen := store.Generation()
	ref := store.CreateMapping([]storage.SeriesRef{1, 2}, lbls, gen)

	require.Zero(t, store.CreateMapping([]storage.SeriesRef{3, 4}, labels.FromStrings("k", "other"), stale))
	require.Len(t, store.uniqueRefToChildRefs, 1)

	require.Nil(t, store.GetMapping(ref, lbls, stale))
	require.Nil(t, store.GetMapping(0, lbls, stale))

	store.UpdateMapping(ref, []storage.SeriesRef{3, 4}, lbls, stale)
	require.Equal(t, []storage.SeriesRef{1, 2}, store.GetMapping(ref, lbls, gen))
}

func TestSeriesRefMappingStore_ClearAlwaysStartsNewGeneration(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)

	// No refs are issued between the two Clears, so the threshold stays put but the
	// generation still moves.
	firstThreshold := store.Clear()
	first := store.Generation()
	secondThreshold := store.Clear()
	require.Equal(t, firstThreshold, secondThreshold)
	require.NotEqual(t, first, store.Generation())
	require.Zero(t, store.CreateMapping([]storage.SeriesRef{1, 2}, labels.FromStrings("k", "v"), first))
}

func TestSeriesRefMappingStore_StopIsIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)

		// Before the cleanup goroutine has started.
		store.Stop()
		store.Stop()

		// A goroutine started after Stop exits straight away, or synctest.Test never returns.
		store.CreateMapping([]storage.SeriesRef{1}, labels.FromStrings("k", "v"), 0)
		store.Stop()
	})
}

func TestSeriesRefMappingStore_ConcurrentReadsAreConsistent(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	// Create a mapping
	childRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(childRefs, lbls, 0)

	// Spawn many goroutines reading the same mapping
	var wg sync.WaitGroup
	numReaders := 100

	for range numReaders {
		wg.Go(func() {
			for range 100 {
				retrieved := store.GetMapping(uniqueRef, lbls, 0)
				require.NotNil(t, retrieved)
				require.Equal(t, childRefs, retrieved)
			}
		})
	}

	wg.Wait()
}

func TestSeriesRefMappingStore_ConcurrentCreatesGetUniqueRefs(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	var wg sync.WaitGroup
	numCreators := 50
	refsPerCreator := 20

	refsChan := make(chan storage.SeriesRef, numCreators*refsPerCreator)

	for i := range numCreators {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := range refsPerCreator {
				childRefs := []storage.SeriesRef{storage.SeriesRef(id*1000 + j)}
				uniqueRef := store.CreateMapping(childRefs, lbls, 0)
				refsChan <- uniqueRef
			}
		}(i)
	}

	wg.Wait()
	close(refsChan)

	// Collect all refs and verify no duplicates
	seenRefs := make(map[storage.SeriesRef]bool)
	count := 0
	for ref := range refsChan {
		require.False(t, seenRefs[ref], "duplicate ref %d", ref)
		seenRefs[ref] = true
		count++
	}

	require.Equal(t, numCreators*refsPerCreator, count)
}

func TestSeriesRefMappingStore_ConcurrentTrackingIsCorrect(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	lbls := labels.EmptyLabels()

	// Create some mappings
	var uniqueRefs []storage.SeriesRef
	for i := range 50 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i)}
		uniqueRef := store.CreateMapping(childRefs, lbls, 0)
		uniqueRefs = append(uniqueRefs, uniqueRef)
	}

	// Track them concurrently from multiple goroutines
	var wg sync.WaitGroup
	numTrackers := 10

	for i := range numTrackers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each tracker tracks a subset of refs
			for j := id; j < len(uniqueRefs); j += numTrackers {
				cell := store.GetCellForAppendedSeries()
				cell.Refs = append(cell.Refs, uniqueRefs[j])
				store.TrackAppendedSeries(time.Now().Unix(), cell)
			}
		}(i)
	}

	wg.Wait()

	// All refs should still be retrievable (tracking shouldn't break anything)
	for _, ref := range uniqueRefs {
		require.NotNil(t, store.GetMapping(ref, lbls, 0))
	}
}

func TestSeriesRefMapping_AppendReusesExistingMapping(t *testing.T) {
	store := newMockMappingStore()
	store.mappingByRef[77] = []storage.SeriesRef{101, 202}

	child1 := &mockAppender{}
	child2 := &mockAppender{}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_reuse", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_reuse", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	lbls := labels.FromStrings("job", "test")
	ref, err := app.Append(77, lbls, 123, 42)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(77), ref)
	require.Equal(t, []storage.SeriesRef{101}, child1.appendRefs)
	require.Equal(t, []storage.SeriesRef{202}, child2.appendRefs)
	require.Len(t, store.createCalls, 0)
	require.Len(t, store.updateCalls, 0)
	require.Equal(t, []storage.SeriesRef{77}, store.cell.Refs)
	require.Equal(t, float64(2), testutil.ToFloat64(samplesForwarded))
}

func TestSeriesRefMapping_AppendUpdatesExistingMappingWhenRefsChange(t *testing.T) {
	store := newMockMappingStore()
	store.mappingByRef[33] = []storage.SeriesRef{11, 22}

	child1 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 111, nil
	}}
	child2 := &mockAppender{}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_update", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_update", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	lbls := labels.FromStrings("job", "test")
	ref, err := app.Append(33, lbls, 123, 42)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(33), ref)
	require.Len(t, store.updateCalls, 1)
	require.Equal(t, storage.SeriesRef(33), store.updateCalls[0].uniqueRef)
	require.Equal(t, []storage.SeriesRef{111, 22}, store.updateCalls[0].refs)
	require.Len(t, store.createCalls, 0)
}

func TestSeriesRefMapping_AppendAllChildrenZeroPassesThroughInputRef(t *testing.T) {
	store := newMockMappingStore()
	zeroFn := func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 0, nil
	}
	child1 := &mockAppender{appendFn: zeroFn}
	child2 := &mockAppender{appendFn: zeroFn}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_all_zero_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_all_zero_forwarded", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	ref, err := app.Append(42, labels.FromStrings("job", "test"), 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(42), ref)
	require.Len(t, store.createCalls, 0)
	require.Empty(t, store.cell.Refs)
}

func TestSeriesRefMapping_AppendUnmappedNonZeroRefForwardsZeroToChildren(t *testing.T) {
	store := newMockMappingStore()

	// Children hand back their own refs regardless of what they're given.
	child1 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 5001, nil
	}}
	child2 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 6002, nil
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_unmapped_nonzero_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_unmapped_nonzero_forwarded", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	// Ref 9999 has no mapping (e.g. a store ref whose mapping was cleaned or predates a
	// topology change). It must not reach the children — they'd risk misattributing the
	// sample to whichever of their own series happens to share that number.
	ref, err := app.Append(9999, labels.FromStrings("job", "test"), 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref)

	require.Equal(t, []storage.SeriesRef{0}, child1.appendRefs)
	require.Equal(t, []storage.SeriesRef{0}, child2.appendRefs)
	require.Len(t, store.createCalls, 1)
	require.Equal(t, []storage.SeriesRef{5001, 6002}, store.createCalls[0].refs)
}

func TestSeriesRefMapping_AppendSingleNonZeroChildCreatesMapping(t *testing.T) {
	store := newMockMappingStore()
	child1 := &mockAppender{}
	child2 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 77, nil
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_single_nonzero_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_single_nonzero_forwarded", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	// A single allocating child still gets a mapping, so the series stays routable on
	// later appends rather than being returned bare and unmapped.
	ref, err := app.Append(0, labels.FromStrings("job", "test"), 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref)
	require.Len(t, store.createCalls, 1)
	require.Equal(t, []storage.SeriesRef{0, 77}, store.createCalls[0].refs)
	require.Equal(t, []storage.SeriesRef{1000}, store.cell.Refs)
}

func TestSeriesRefMapping_AppendSecondAppendUsesChildRefsFromMapping(t *testing.T) {
	store := newMockMappingStore()

	// Both children return non-zero so a mapping is created on the first append.
	child1 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 5001, nil
	}}
	child2 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 77, nil
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_single_no_leak", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_single_no_leak", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	lbls := labels.FromStrings("job", "single")

	// First append: both children return non-zero, so a mapping is created.
	ref, err := app.Append(0, lbls, 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref)

	// Second append: mapping is found and each child is called with its stored child ref.
	_, err = app.Append(ref, lbls, 2, 2)
	require.NoError(t, err)
	require.Equal(t, []storage.SeriesRef{0, 5001}, child1.appendRefs)
	require.Equal(t, []storage.SeriesRef{0, 77}, child2.appendRefs)
}

func TestSeriesRefMapping_AppendErrorSkipsMappingUpdate(t *testing.T) {
	store := newMockMappingStore()
	store.mappingByRef[88] = []storage.SeriesRef{11, 22}

	child1 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 111, nil
	}}
	child2 := &mockAppender{appendFn: func(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return ref, errors.New("child append failed")
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_error", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_error", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	ref, err := app.Append(88, labels.EmptyLabels(), 1, 1)
	require.Error(t, err)
	require.Equal(t, storage.SeriesRef(0), ref)
	require.Len(t, store.updateCalls, 0)
	require.Len(t, store.createCalls, 0)
	require.Equal(t, float64(1), testutil.ToFloat64(samplesForwarded))
}

func TestSeriesRefMapping_CommitTracksRefsAndAggregatesErrors(t *testing.T) {
	store := newMockMappingStore()
	store.mappingByRef[101] = []storage.SeriesRef{11, 22}

	child1 := &mockAppender{commitFn: func() error { return errors.New("child1 commit failed") }}
	child2 := &mockAppender{commitFn: func() error { return errors.New("child2 commit failed") }}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_commit", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_commit", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	_, err := app.Append(101, labels.EmptyLabels(), 1, 1)
	require.NoError(t, err)

	err = app.Commit()
	require.ErrorContains(t, err, "child1 commit failed")
	require.ErrorContains(t, err, "child2 commit failed")
	require.Len(t, store.trackCalls, 1)
	require.Equal(t, []storage.SeriesRef{101}, store.trackCalls[0].refs)
	require.Empty(t, store.cell.Refs)
	require.Equal(t, 1, child1.commitCalls)
	require.Equal(t, 1, child2.commitCalls)
}

func TestSeriesRefMapping_RollbackTracksRefs(t *testing.T) {
	store := newMockMappingStore()
	store.mappingByRef[202] = []storage.SeriesRef{33, 44}

	child1 := &mockAppender{rollbackFn: func() error { return nil }}
	child2 := &mockAppender{rollbackFn: func() error { return errors.New("child2 rollback failed") }}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_rollback", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_rollback", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	_, err := app.Append(202, labels.EmptyLabels(), 1, 1)
	require.NoError(t, err)

	err = app.Rollback()
	require.ErrorContains(t, err, "child2 rollback failed")
	require.Len(t, store.trackCalls, 1)
	require.Equal(t, []storage.SeriesRef{202}, store.trackCalls[0].refs)
	require.Equal(t, 1, child1.rollbackCalls)
	require.Equal(t, 1, child2.rollbackCalls)
}

// TestSeriesRefMapping_MappingReusedOnSubsequentAppends verifies that the second
// append for a series uses the existing mapping and does not create a new one.
func TestSeriesRefMapping_MappingReusedOnSubsequentAppends(t *testing.T) {
	store := newMockMappingStore()

	child1 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 5001, nil
	}}
	child2 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 6002, nil
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_reuse_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_reuse_forwarded", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	lbls := labels.FromStrings("job", "test")

	// First append: no existing mapping, creates one.
	ref, err := app.Append(0, lbls, 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref)
	require.Len(t, store.createCalls, 1)

	// Second append: existing mapping is found and reused, no new create.
	ref2, err := app.Append(ref, lbls, 2, 2)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(1000), ref2)
	require.Len(t, store.createCalls, 1, "no new mapping should be created on second append")
	require.Len(t, store.updateCalls, 0)
}

// TestSeriesRefMapping_ChildRefChangeUpdatesMapping verifies that when a child
// returns a different ref than the one stored in the mapping, the mapping is
// updated so subsequent appends use the new child ref.
func TestSeriesRefMapping_ChildRefChangeUpdatesMapping(t *testing.T) {
	store := newMockMappingStore()

	// child2 starts at 6002 then changes to 9999 on the second append.
	child2Calls := 0
	child1 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 5001, nil
	}}
	child2 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		child2Calls++
		if child2Calls == 1 {
			return 6002, nil
		}
		return 9999, nil
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_child_change_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_series_ref_mapping_child_change_forwarded", Help: "test"})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, 0, writeLatency, samplesForwarded)

	lbls := labels.FromStrings("job", "test")

	// First append: creates mapping with [5001, 6002].
	ref, err := app.Append(0, lbls, 1, 1)
	require.NoError(t, err)
	require.Len(t, store.createCalls, 1)
	require.Equal(t, []storage.SeriesRef{5001, 6002}, store.createCalls[0].refs)

	// Second append: child2 returns 9999 instead of 6002, mapping should be updated.
	_, err = app.Append(ref, lbls, 2, 2)
	require.NoError(t, err)
	require.Len(t, store.updateCalls, 1)
	require.Equal(t, []storage.SeriesRef{5001, 9999}, store.updateCalls[0].refs)

	// Third append: child2 is called with updated ref 9999.
	_, err = app.Append(ref, lbls, 3, 3)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(9999), child2.appendRefs[2])
}

// TestSeriesRefMapping_AppenderFromPreviousGenerationIsIsolated covers an appender built
// before a children change that is still appending after it.
func TestSeriesRefMapping_AppenderFromPreviousGenerationIsIsolated(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Stop)
	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_prev_gen_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_prev_gen_forwarded", Help: "test"})
	childReturning := func(ref storage.SeriesRef) *mockAppender {
		return &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
			return ref, nil
		}}
	}

	oldA, oldB := childReturning(11), childReturning(22)
	oldApp := NewSeriesRefMapping([]storage.Appender{oldA, oldB}, store, store.Generation(), writeLatency, samplesForwarded)
	store.Clear()
	gen := store.Generation()
	newC, newD := childReturning(33), childReturning(44)
	newApp := NewSeriesRefMapping([]storage.Appender{newC, newD}, store, gen, writeLatency, samplesForwarded)

	// The old appender's child refs never reach the store.
	written := labels.FromStrings("job", "written")
	ref, err := oldApp.Append(0, written, 1, 1)
	require.NoError(t, err)
	require.Zero(t, ref)
	_, err = newApp.Append(0, written, 1, 1)
	require.NoError(t, err)
	require.Equal(t, []storage.SeriesRef{0}, newC.appendRefs)
	require.Equal(t, []storage.SeriesRef{0}, newD.appendRefs)

	// Nor do the new children's refs reach the old appender's children.
	read := labels.FromStrings("job", "read")
	ref, err = newApp.Append(0, read, 1, 1)
	require.NoError(t, err)
	_, err = oldApp.Append(ref, read, 1, 1)
	require.NoError(t, err)
	_, err = oldApp.Append(0, read, 1, 1)
	require.NoError(t, err)
	require.Equal(t, []storage.SeriesRef{0, 0, 0}, oldA.appendRefs)
	require.Equal(t, []storage.SeriesRef{0, 0, 0}, oldB.appendRefs)

	// The new mapping still holds the new children's refs.
	require.Equal(t, []storage.SeriesRef{33, 44}, store.GetMapping(ref, read, gen))
}

type createCall struct {
	refs []storage.SeriesRef
	lbls labels.Labels
}

type updateCall struct {
	uniqueRef storage.SeriesRef
	refs      []storage.SeriesRef
	lbls      labels.Labels
}

type trackCall struct {
	ts   int64
	refs []storage.SeriesRef
}

type mockMappingStore struct {
	mappingByRef  map[storage.SeriesRef][]storage.SeriesRef
	mappingByHash map[uint64]storage.SeriesRef
	createCalls   []createCall
	updateCalls   []updateCall
	trackCalls    []trackCall
	createRef     storage.SeriesRef
	cell          *Cell
}

func newMockMappingStore() *mockMappingStore {
	return &mockMappingStore{
		mappingByRef:  map[storage.SeriesRef][]storage.SeriesRef{},
		mappingByHash: map[uint64]storage.SeriesRef{},
		createRef:     1000,
		cell:          &Cell{Refs: make([]storage.SeriesRef, 0, 10)},
	}
}

func (m *mockMappingStore) GetMapping(uniqueRef storage.SeriesRef, lbls labels.Labels, _ uint64) []storage.SeriesRef {
	if uniqueRef == 0 {
		mappedRef, ok := m.mappingByHash[lbls.Hash()]
		if !ok {
			return nil
		}
		uniqueRef = mappedRef
	}

	refs, ok := m.mappingByRef[uniqueRef]
	if !ok {
		return nil
	}

	return copyRefs(refs)
}

func (m *mockMappingStore) CreateMapping(refResults []storage.SeriesRef, lbls labels.Labels, _ uint64) storage.SeriesRef {
	newRef := m.createRef
	m.createRef++

	copiedRefs := copyRefs(refResults)
	m.mappingByRef[newRef] = copiedRefs
	m.mappingByHash[lbls.Hash()] = newRef
	m.createCalls = append(m.createCalls, createCall{refs: copiedRefs, lbls: lbls})

	return newRef
}

func (m *mockMappingStore) UpdateMapping(uniqueRef storage.SeriesRef, refResults []storage.SeriesRef, lbls labels.Labels, _ uint64) {
	copiedRefs := copyRefs(refResults)
	m.mappingByRef[uniqueRef] = copiedRefs
	m.mappingByHash[lbls.Hash()] = uniqueRef
	m.updateCalls = append(m.updateCalls, updateCall{uniqueRef: uniqueRef, refs: copiedRefs, lbls: lbls})
}

func (m *mockMappingStore) TrackAppendedSeries(ts int64, cell *Cell) {
	m.trackCalls = append(m.trackCalls, trackCall{ts: ts, refs: copyRefs(cell.Refs)})
	cell.Refs = cell.Refs[:0]
}

func (m *mockMappingStore) GetCellForAppendedSeries() *Cell {
	return m.cell
}

func copyRefs(in []storage.SeriesRef) []storage.SeriesRef {
	out := make([]storage.SeriesRef, len(in))
	copy(out, in)
	return out
}

type mockAppender struct {
	appendFn                      func(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error)
	appendExemplarFn              func(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error)
	appendHistogramFn             func(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error)
	appendHistogramSTZeroSampleFn func(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error)
	updateMetadataFn              func(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error)
	appendSTZeroSampleFn          func(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error)
	commitFn                      func() error
	rollbackFn                    func() error
	setOptionsFn                  func(opts *storage.AppendOptions)

	appendRefs    []storage.SeriesRef
	commitCalls   int
	rollbackCalls int
}

func (m *mockAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	m.appendRefs = append(m.appendRefs, ref)
	if m.appendFn != nil {
		return m.appendFn(ref, l, t, v)
	}
	return ref, nil
}

func (m *mockAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	if m.appendExemplarFn != nil {
		return m.appendExemplarFn(ref, l, e)
	}
	return ref, nil
}

func (m *mockAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if m.appendHistogramFn != nil {
		return m.appendHistogramFn(ref, l, t, h, fh)
	}
	return ref, nil
}

func (m *mockAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if m.appendHistogramSTZeroSampleFn != nil {
		return m.appendHistogramSTZeroSampleFn(ref, l, t, st, h, fh)
	}
	return ref, nil
}

func (m *mockAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, md metadata.Metadata) (storage.SeriesRef, error) {
	if m.updateMetadataFn != nil {
		return m.updateMetadataFn(ref, l, md)
	}
	return ref, nil
}

func (m *mockAppender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	if m.appendSTZeroSampleFn != nil {
		return m.appendSTZeroSampleFn(ref, l, t, st)
	}
	return ref, nil
}

func (m *mockAppender) Commit() error {
	m.commitCalls++
	if m.commitFn != nil {
		return m.commitFn()
	}
	return nil
}

func (m *mockAppender) Rollback() error {
	m.rollbackCalls++
	if m.rollbackFn != nil {
		return m.rollbackFn()
	}
	return nil
}

func (m *mockAppender) SetOptions(opts *storage.AppendOptions) {
	if m.setOptionsFn != nil {
		m.setOptionsFn(opts)
	}
}
