package wal

import (
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"
)

func TestNoDeadlock(t *testing.T) {
	const numWorkers = 1000

	var (
		wg           sync.WaitGroup
		started      = make(chan struct{})
		stripeSeries = newStripeSeries(3)
	)

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			<-started
			_ = stripeSeries.gc(math.MaxInt64)
		}()
	}

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(i int) {
			defer wg.Done()
			<-started

			series := &memSeries{
				ref: chunks.HeadSeriesRef(i),
				lset: labels.FromMap(map[string]string{
					"id": strconv.Itoa(i),
				}),
			}
			stripeSeries.Set(series.lset.Hash(), series)
		}(i)
	}

	finished := make(chan struct{})
	go func() {
		wg.Wait()
		close(finished)
	}()

	close(started)
	select {
	case <-finished:
		return
	case <-time.After(15 * time.Second):
		require.FailNow(t, "deadlock detected")
	}
}

func labelsWithHashCollision() (labels.Labels, labels.Labels) {
	// These two series have the same XXHash; thanks to https://github.com/pstibrany/labels_hash_collisions
	ls1 := labels.FromStrings("__name__", "metric", "lbl", "HFnEaGl")
	ls2 := labels.FromStrings("__name__", "metric", "lbl", "RqcXatm")

	if ls1.Hash() != ls2.Hash() {
		// These ones are the same when using -tags slicelabels
		ls1 = labels.FromStrings("__name__", "metric", "lbl1", "value", "lbl2", "l6CQ5y")
		ls2 = labels.FromStrings("__name__", "metric", "lbl1", "value", "lbl2", "v7uDlF")
	}

	if ls1.Hash() != ls2.Hash() {
		panic("This code needs to be updated: find new labels with colliding hash values.")
	}

	return ls1, ls2
}

// stripeSeriesWithCollidingSeries returns a stripeSeries with two memSeries having the same, colliding, hash.
func stripeSeriesWithCollidingSeries() (*stripeSeries, *memSeries, *memSeries) {
	lbls1, lbls2 := labelsWithHashCollision()
	ms1 := memSeries{
		lset: lbls1,
	}
	ms2 := memSeries{
		lset: lbls2,
	}
	hash := lbls1.Hash()
	s := newStripeSeries(1)

	s.Set(hash, &ms1)
	s.Set(hash, &ms2)

	return s, &ms1, &ms2
}

func TestStripeSeries_Get(t *testing.T) {
	s, ms1, ms2 := stripeSeriesWithCollidingSeries()
	hash := ms1.lset.Hash()

	// Verify that we can get both of the series despite the hash collision
	got := s.GetByHash(hash, ms1.lset)
	require.Same(t, ms1, got)
	got = s.GetByHash(hash, ms2.lset)
	require.Same(t, ms2, got)
}

func TestStripeSeries_gc(t *testing.T) {
	s, ms1, ms2 := stripeSeriesWithCollidingSeries()
	hash := ms1.lset.Hash()

	s.gc(1)

	// Verify that we can get neither ms1 nor ms2 after gc-ing corresponding series
	got := s.GetByHash(hash, ms1.lset)
	require.Nil(t, got)
	got = s.GetByHash(hash, ms2.lset)
	require.Nil(t, got)
}

func TestSeriesHashmap_Flow(t *testing.T) {
	l1, l2 := labelsWithHashCollision()
	hm := seriesHashmap{
		unique:    map[uint64]*memSeries{},
		conflicts: nil,
	}

	hash := l1.Hash()

	// Make sure we can set and get m1
	expectedM1Ref := chunks.HeadSeriesRef(1)
	hm.Set(hash, &memSeries{lset: l1, ref: expectedM1Ref})
	m1 := hm.Get(hash, l1)
	require.NotNil(t, m1)
	require.Equal(t, expectedM1Ref, m1.ref)

	// Add a collision as m2 and make sure we can get it
	expectedM2Ref := chunks.HeadSeriesRef(2)
	hm.Set(hash, &memSeries{lset: l2, ref: expectedM2Ref})
	m2 := hm.Get(hash, l2)
	require.NotNil(t, m2)
	require.Equal(t, expectedM2Ref, m2.ref)

	// Make sure m1 is unchanged
	m1Again := hm.Get(hash, l1)
	require.Same(t, m1, m1Again)

	// Delete the collision m2
	hm.Delete(hash, expectedM2Ref)

	// Make sure m2 is gone and m1 is uneffected
	m2 = hm.Get(hash, l2)
	require.Nil(t, m2)
	m1Again = hm.Get(hash, l1)
	require.Same(t, m1, m1Again)

	// Add m2 back again
	hm.Set(hash, &memSeries{lset: l2, ref: expectedM2Ref})

	// Delete the unique m1 make sure m2 is unaffected and m1 is gone
	hm.Delete(hash, expectedM1Ref)
	m1 = hm.Get(hash, l1)
	require.Nil(t, m1)
	m2Again := hm.Get(hash, l1)
	require.Same(t, m2, m2Again)

	// Delete m2 and make sure it's gone
	hm.Delete(hash, expectedM2Ref)
	m2 = hm.Get(hash, l1)
	require.Nil(t, m2)
}
