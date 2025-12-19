package appenders

import (
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestGetMappingReturnsNilForUnknownRef(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	require.Nil(t, store.GetMapping(0))
	require.Nil(t, store.GetMapping(1))
	require.Nil(t, store.GetMapping(999))
	require.Nil(t, store.GetMapping(storage.SeriesRef(12345)))
}

func TestCreatedMappingCanBeRetrieved(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

	childRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(childRefs)

	retrieved := store.GetMapping(uniqueRef)
	require.NotNil(t, retrieved)
	require.Equal(t, childRefs, retrieved)
}

func TestEachCreatedMappingGetsUniqueRef(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

	refs := make(map[storage.SeriesRef]bool)
	mappings := make(map[storage.SeriesRef][]storage.SeriesRef)

	for i := range 100 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i), storage.SeriesRef(i + 1)}
		uniqueRef := store.CreateMapping(childRefs)

		// Verify this ref is unique
		require.False(t, refs[uniqueRef], "ref %d was already used", uniqueRef)
		refs[uniqueRef] = true
		mappings[uniqueRef] = childRefs
	}

	// Verify all mappings can be retrieved independently
	for uniqueRef, expectedChildRefs := range mappings {
		retrieved := store.GetMapping(uniqueRef)
		require.Equal(t, expectedChildRefs, retrieved)
	}
}

func TestUpdateMappingChangesReturnedValue(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

	originalRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(originalRefs)

	updatedRefs := []storage.SeriesRef{4, 5, 6}
	store.UpdateMapping(uniqueRef, updatedRefs)

	retrieved := store.GetMapping(uniqueRef)
	require.Equal(t, updatedRefs, retrieved)
	require.NotEqual(t, originalRefs, retrieved)
}

func TestUpdateMappingWithZeroRefDoesNothing(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	store.UpdateMapping(0, []storage.SeriesRef{1, 2, 3})

	// Should still return nil
	require.Nil(t, store.GetMapping(0))
}

func TestTrackAppendedSeriesDoesNotPanic(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	slice := store.GetSliceForAppendedSeries()
	slice = append(slice, 1, 2, 3)

	require.NotPanics(t, func() {
		store.TrackAppendedSeries(time.Now().Unix(), slice)
	})
}

func TestSliceIsEmptyAfterReturn(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	slice1 := store.GetSliceForAppendedSeries()
	slice1 = append(slice1, 1, 2, 3)
	store.TrackAppendedSeries(time.Now().Unix(), slice1)

	slice2 := store.GetSliceForAppendedSeries()
	require.NotNil(t, slice2)
	require.Equal(t, 0, len(slice2), "slice returned should always have length 0")
}

func TestRefsAreEventuallyCleanedUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Clear)

		// Create and track a mapping with old timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs)

		oldTimestamp := time.Now().Add(-20 * time.Minute).Unix()
		slice := store.GetSliceForAppendedSeries()
		slice = append(slice, uniqueRef)
		store.TrackAppendedSeries(oldTimestamp, slice)

		// Verify mapping exists initially
		require.NotNil(t, store.GetMapping(uniqueRef))

		// Wait for cleanup to run (15 minute ticker + some buffer)
		time.Sleep(16 * time.Minute)

		// Mapping should be cleaned up
		require.Nil(t, store.GetMapping(uniqueRef))
	})
}

func TestRecentlyTrackedRefsAreNotCleanedUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Clear)

		// Create and track a mapping with recent timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs)

		recentTimestamp := time.Now().Unix()
		slice := store.GetSliceForAppendedSeries()
		slice = append(slice, uniqueRef)
		store.TrackAppendedSeries(recentTimestamp, slice)

		// Wait for a cleanup cycle
		time.Sleep(16 * time.Minute)

		// Mapping should still exist
		require.NotNil(t, store.GetMapping(uniqueRef))
	})
}

func TestTrackingRefAgainUpdatesTimestamp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewSeriesRefMappingStore(nil)
		t.Cleanup(store.Clear)

		// Create and track a mapping with old timestamp
		childRefs := []storage.SeriesRef{1, 2, 3}
		uniqueRef := store.CreateMapping(childRefs)

		oldTimestamp := time.Now().Add(-20 * time.Minute).Unix()
		slice1 := store.GetSliceForAppendedSeries()
		slice1 = append(slice1, uniqueRef)
		store.TrackAppendedSeries(oldTimestamp, slice1)

		// Wait a bit
		time.Sleep(1 * time.Minute)

		// Track the same ref again with current timestamp
		currentTimestamp := time.Now().Unix()
		slice2 := store.GetSliceForAppendedSeries()
		slice2 = append(slice2, uniqueRef)
		store.TrackAppendedSeries(currentTimestamp, slice2)

		// Wait for cleanup cycle
		time.Sleep(16 * time.Minute)

		// Mapping should NOT be cleaned up because timestamp was refreshed
		require.NotNil(t, store.GetMapping(uniqueRef))
	})
}

func TestClearRemovesAllMappings(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	// Create several mappings
	var uniqueRefs []storage.SeriesRef
	for i := range 10 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i), storage.SeriesRef(i + 1)}
		uniqueRef := store.CreateMapping(childRefs)
		uniqueRefs = append(uniqueRefs, uniqueRef)

		// Track them
		slice := store.GetSliceForAppendedSeries()
		slice = append(slice, uniqueRef)
		store.TrackAppendedSeries(time.Now().Unix(), slice)
	}

	// Verify they exist
	for _, ref := range uniqueRefs {
		require.NotNil(t, store.GetMapping(ref))
	}

	// Clear
	store.Clear()

	// Verify all are gone
	for _, ref := range uniqueRefs {
		require.Nil(t, store.GetMapping(ref))
	}
}

func TestClearIsIdempotent(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)

	// Create some mappings
	childRefs := []storage.SeriesRef{1, 2, 3}
	store.CreateMapping(childRefs)

	// Clear multiple times
	require.NotPanics(t, func() {
		store.Clear()
		store.Clear()
		store.Clear()
	})
}

func TestStoreCanBeReusedAfterClear(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

	// Create multiple mappings before clear
	childRefs1 := []storage.SeriesRef{1, 2, 3}
	uniqueRef1 := store.CreateMapping(childRefs1)
	childRefs2 := []storage.SeriesRef{7, 8, 9}
	uniqueRef2 := store.CreateMapping(childRefs2)

	// Clear
	store.Clear()

	// Create new mappings after clear
	childRefs3 := []storage.SeriesRef{4, 5, 6}
	uniqueRef3 := store.CreateMapping(childRefs3)

	// New mapping should work
	retrieved := store.GetMapping(uniqueRef3)
	require.NotNil(t, retrieved)
	require.Equal(t, childRefs3, retrieved)

	// Old mappings should not exist
	require.Nil(t, store.GetMapping(uniqueRef1))
	require.Nil(t, store.GetMapping(uniqueRef2))
}

// Concurrent API Usage Tests

func TestConcurrentReadsAreConsistent(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

	// Create a mapping
	childRefs := []storage.SeriesRef{1, 2, 3}
	uniqueRef := store.CreateMapping(childRefs)

	// Spawn many goroutines reading the same mapping
	var wg sync.WaitGroup
	numReaders := 100

	for range numReaders {
		wg.Go(func() {
			for range 100 {
				retrieved := store.GetMapping(uniqueRef)
				require.NotNil(t, retrieved)
				require.Equal(t, childRefs, retrieved)
			}
		})
	}

	wg.Wait()
}

func TestConcurrentCreatesGetUniqueRefs(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

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
				uniqueRef := store.CreateMapping(childRefs)
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

func TestConcurrentTrackingIsCorrect(t *testing.T) {
	store := NewSeriesRefMappingStore(nil)
	t.Cleanup(store.Clear)

	// Create some mappings
	var uniqueRefs []storage.SeriesRef
	for i := range 50 {
		childRefs := []storage.SeriesRef{storage.SeriesRef(i)}
		uniqueRef := store.CreateMapping(childRefs)
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
				slice := store.GetSliceForAppendedSeries()
				slice = append(slice, uniqueRefs[j])
				store.TrackAppendedSeries(time.Now().Unix(), slice)
			}
		}(i)
	}

	wg.Wait()

	// All refs should still be retrievable (tracking shouldn't break anything)
	for _, ref := range uniqueRefs {
		require.NotNil(t, store.GetMapping(ref))
	}
}
