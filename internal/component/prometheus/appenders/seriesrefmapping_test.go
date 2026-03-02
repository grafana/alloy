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

	require.Nil(t, store.GetMapping(0, labels.EmptyLabels()))
	require.Nil(t, store.GetMapping(1, labels.EmptyLabels()))
	require.Nil(t, store.GetMapping(999, labels.EmptyLabels()))
	require.Nil(t, store.GetMapping(storage.SeriesRef(12345), labels.EmptyLabels()))
}

func TestSeriesRefMappingStore_CreatedMappingCanBeRetrieved(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })

	childRefs := []storage.SeriesRef{1, 2, 3}
	lbls := labels.NewBuilder(labels.EmptyLabels()).Set("foo", "bar").Labels()

	uniqueRef := store.CreateMapping(childRefs, lbls)

	// Case 1: get by unique ref
	got := store.GetMapping(uniqueRef, lbls)
	require.NotNil(t, got)
	require.Equal(t, childRefs, got)

	// Case 1: rely on label hash fallback
	got = store.GetMapping(0, lbls)
	require.NotNil(t, got)
	require.Equal(t, childRefs, got)
}

func TestSeriesRefMappingStore_EachCreatedMappingGetsUniqueRef(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })

	type mappingAndLabels struct {
		refs   []storage.SeriesRef
		labels labels.Labels
	}

	refs := make(map[storage.SeriesRef]bool)
	mappings := make(map[storage.SeriesRef]mappingAndLabels)

	for i := range 100 {
		lbls := labels.NewBuilder(labels.EmptyLabels()).Set("k", strconv.Itoa(i)).Labels()
		childRefs := []storage.SeriesRef{storage.SeriesRef(i), storage.SeriesRef(i + 1)}
		uniqueRef := store.CreateMapping(childRefs, lbls)

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
		retrieved := store.GetMapping(uniqueRef, mp.labels)
		require.Equal(t, mp.refs, retrieved)
	}
}

func TestSeriesRefMappingStore_UpdateMappingChangesReturnedValue(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })
	lbls := labels.EmptyLabels()

	originalRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(originalRefs, lbls)

	updatedRefs := []storage.SeriesRef{4, 5, 6}
	store.UpdateMapping(uniqueRef, updatedRefs, lbls)

	retrieved := store.GetMapping(uniqueRef, lbls)
	require.Equal(t, updatedRefs, retrieved)
	require.NotEqual(t, originalRefs, retrieved)
}

func TestSeriesRefMappingStore_UpdateMappingWithZeroRefDoesNothing(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	lbls := labels.EmptyLabels()

	store.UpdateMapping(0, []storage.SeriesRef{1, 2, 3}, lbls)

	// Should still return nil
	require.Nil(t, store.GetMapping(0, lbls))
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
		t.Cleanup(func() { store.Clear() })
		lbls := labels.EmptyLabels()

		// Create and track a mapping with old timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs, lbls)

		oldTimestamp := time.Now().Add(-20 * time.Minute).Unix()
		cell := store.GetCellForAppendedSeries()
		cell.Refs = append(cell.Refs, uniqueRef)
		store.TrackAppendedSeries(oldTimestamp, cell)

		// Verify mapping exists initially
		require.NotNil(t, store.GetMapping(uniqueRef, lbls))

		// Wait for cleanup to run (15 minute ticker + some buffer)
		time.Sleep(16 * time.Minute)

		// Mapping should be cleaned up
		require.Nil(t, store.GetMapping(uniqueRef, lbls))
	})
}

func TestSeriesRefMappingStore_RecentlyTrackedRefsAreNotCleanedUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(func() { store.Clear() })
		lbls := labels.EmptyLabels()

		// Create and track a mapping with recent timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs, lbls)

		recentTimestamp := time.Now().Unix()
		cell := store.GetCellForAppendedSeries()
		cell.Refs = append(cell.Refs, uniqueRef)
		store.TrackAppendedSeries(recentTimestamp, cell)

		// Wait for a cleanup cycle
		time.Sleep(16 * time.Minute)

		// Mapping should still exist
		require.NotNil(t, store.GetMapping(uniqueRef, lbls))
	})
}

func TestSeriesRefMappingStore_TrackingRefAgainUpdatesTimestamp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(func() { store.Clear() })
		lbls := labels.EmptyLabels()

		// Create and track a mapping with old timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs, lbls)

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
		require.NotNil(t, store.GetMapping(uniqueRef, lbls))
	})
}

func TestSeriesRefMappingStore_ClearRemovesAllMappings(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	lbls := labels.EmptyLabels()

	// Create several mappings
	var uniqueRefs []storage.SeriesRef
	for i := range 10 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i), storage.SeriesRef(i + 1)}
		uniqueRef := store.CreateMapping(childRefs, lbls)
		uniqueRefs = append(uniqueRefs, uniqueRef)

		// Track them
		cell := store.GetCellForAppendedSeries()
		cell.Refs = append(cell.Refs, uniqueRef)
		store.TrackAppendedSeries(time.Now().Unix(), cell)
	}

	// Verify they exist
	for _, ref := range uniqueRefs {
		require.NotNil(t, store.GetMapping(ref, lbls))
	}

	// Clear advances the generation boundary past all previously issued refs.
	threshold := store.Clear()
	require.Greater(t, uint64(threshold), uint64(uniqueRefs[len(uniqueRefs)-1]),
		"threshold must be above all previously issued refs")

	// Verify all are gone
	for _, ref := range uniqueRefs {
		require.Nil(t, store.GetMapping(ref, lbls))
	}
}

func TestSeriesRefMappingStore_ClearIsIdempotent(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	lbls := labels.EmptyLabels()

	// Create some mappings
	childRefs := []storage.SeriesRef{1, 2, 3}
	store.CreateMapping(childRefs, lbls)

	// Clear multiple times
	require.NotPanics(t, func() {
		store.Clear()
		store.Clear()
		store.Clear()
	})
}

func TestSeriesRefMappingStore_CanBeReusedAfterClear(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })
	lbls := labels.EmptyLabels()

	// Create multiple mappings before clear
	childRefs1 := []storage.SeriesRef{1, 2, 3}
	uniqueRef1 := store.CreateMapping(childRefs1, lbls)
	childRefs2 := []storage.SeriesRef{7, 8, 9}
	uniqueRef2 := store.CreateMapping(childRefs2, lbls)

	// Clear
	store.Clear()

	// Create new mappings after clear
	childRefs3 := []storage.SeriesRef{4, 5, 6}
	uniqueRef3 := store.CreateMapping(childRefs3, lbls)

	// New mapping should work
	retrieved := store.GetMapping(uniqueRef3, lbls)
	require.NotNil(t, retrieved)
	require.Equal(t, childRefs3, retrieved)

	// Old mappings should not exist
	require.Nil(t, store.GetMapping(uniqueRef1, lbls))
	require.Nil(t, store.GetMapping(uniqueRef2, lbls))
}

func TestSeriesRefMappingStore_ConcurrentReadsAreConsistent(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })
	lbls := labels.EmptyLabels()

	// Create a mapping
	childRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(childRefs, lbls)

	// Spawn many goroutines reading the same mapping
	var wg sync.WaitGroup
	numReaders := 100

	for range numReaders {
		wg.Go(func() {
			for range 100 {
				retrieved := store.GetMapping(uniqueRef, lbls)
				require.NotNil(t, retrieved)
				require.Equal(t, childRefs, retrieved)
			}
		})
	}

	wg.Wait()
}

func TestSeriesRefMappingStore_ConcurrentCreatesGetUniqueRefs(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(func() { store.Clear() })
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
				uniqueRef := store.CreateMapping(childRefs, lbls)
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
	t.Cleanup(func() { store.Clear() })
	lbls := labels.EmptyLabels()

	// Create some mappings
	var uniqueRefs []storage.SeriesRef
	for i := range 50 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i)}
		uniqueRef := store.CreateMapping(childRefs, lbls)
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
		require.NotNil(t, store.GetMapping(ref, lbls))
	}
}

func TestSeriesRefMapping_AppendReusesExistingMapping(t *testing.T) {
	store := newMockMappingStore()
	store.mappingByRef[77] = []storage.SeriesRef{101, 202}

	child1 := &mockAppender{}
	child2 := &mockAppender{}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_series_ref_mapping_write_latency_reuse", Help: "test"})
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_reuse", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_update", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_all_zero_forwarded", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

	ref, err := app.Append(42, labels.FromStrings("job", "test"), 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(42), ref)
	require.Len(t, store.createCalls, 0)
	require.Empty(t, store.cell.Refs)
}

func TestSeriesRefMapping_AppendSingleNonZeroChildReturnsChildRefDirectly(t *testing.T) {
	store := newMockMappingStore()
	child1 := &mockAppender{}
	child2 := &mockAppender{appendFn: func(_ storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
		return 77, nil
	}}

	writeLatency := prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_single_nonzero_latency", Help: "test"})
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_single_nonzero_forwarded", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

	// The single non-zero child ref is returned directly — no mapping created.
	ref, err := app.Append(0, labels.FromStrings("job", "test"), 1, 1)
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(77), ref)
	require.Len(t, store.createCalls, 0)
	require.Empty(t, store.cell.Refs)
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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_single_no_leak", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_error", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_commit", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_samples_forwarded_rollback", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_reuse_forwarded", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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
	samplesForwarded := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_series_ref_mapping_child_change_forwarded", Help: "test"}, []string{})
	app := NewSeriesRefMapping([]storage.Appender{child1, child2}, store, writeLatency, samplesForwarded)

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

func (m *mockMappingStore) GetMapping(uniqueRef storage.SeriesRef, lbls labels.Labels) []storage.SeriesRef {
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

func (m *mockMappingStore) CreateMapping(refResults []storage.SeriesRef, lbls labels.Labels) storage.SeriesRef {
	newRef := m.createRef
	m.createRef++

	copiedRefs := copyRefs(refResults)
	m.mappingByRef[newRef] = copiedRefs
	m.mappingByHash[lbls.Hash()] = newRef
	m.createCalls = append(m.createCalls, createCall{refs: copiedRefs, lbls: lbls})

	return newRef
}

func (m *mockMappingStore) UpdateMapping(uniqueRef storage.SeriesRef, refResults []storage.SeriesRef, lbls labels.Labels) {
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
