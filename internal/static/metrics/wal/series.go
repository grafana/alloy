package wal

// This code is copied from
// prometheus/prometheus@7c2de14b0bd74303c2ca6f932b71d4585a29ca75, with only
// minor changes for metric names.

import (
	"sync"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"go.uber.org/atomic"
)

// memSeries is a chunkless version of tsdb.memSeries.
type memSeries struct {
	sync.Mutex

	ref  chunks.HeadSeriesRef
	lset labels.Labels

	// Last recorded timestamp. Used by gc to determine if a series is stale.
	lastTs int64
}

// updateTimestamp obtains the lock on s and will attempt to update lastTs.
// fails if newTs < lastTs.
func (m *memSeries) updateTimestamp(newTs int64) bool {
	m.Lock()
	defer m.Unlock()
	if newTs >= m.lastTs {
		m.lastTs = newTs
		return true
	}
	return false
}

// seriesHashmap is a simple hashmap for memSeries by their label set.
// It is built on top of a regular hashmap and holds a slice of series to
// resolve hash collisions. Its methods require the hash to be submitted
// with the label set to avoid re-computing hash throughout the code.
type seriesHashmap map[uint64][]*memSeries

func (m seriesHashmap) Get(hash uint64, lset labels.Labels) *memSeries {
	for _, s := range m[hash] {
		if labels.Equal(s.lset, lset) {
			return s
		}
	}
	return nil
}

func (m seriesHashmap) Set(hash uint64, s *memSeries) {
	seriesSet := m[hash]
	for i, prev := range seriesSet {
		if labels.Equal(prev.lset, s.lset) {
			seriesSet[i] = s
			return
		}
	}
	m[hash] = append(seriesSet, s)
}

func (m seriesHashmap) Delete(hash uint64, ref chunks.HeadSeriesRef) {
	var rem []*memSeries
	for _, s := range m[hash] {
		if s.ref != ref {
			rem = append(rem, s)
		}
	}
	if len(rem) == 0 {
		delete(m, hash)
	} else {
		m[hash] = rem
	}
}

// stripeSeries locks modulo ranges of IDs and hashes to reduce lock
// contention. The locks are padded to not be on the same cache line.
// Filling the padded space with the maps was profiled to be slower -
// likely due to the additional pointer dereferences.
type stripeSeries struct {
	size        int
	series      []map[chunks.HeadSeriesRef]*memSeries
	hashes      []seriesHashmap
	exemplars   []map[chunks.HeadSeriesRef]*exemplar.Exemplar
	locks       []stripeLock
	initialized *atomic.Bool

	gcMut sync.Mutex
}

type stripeLock struct {
	sync.RWMutex
	// Padding to avoid multiple locks being on the same cache line.
	_ [40]byte
}

// newStripeSeries creates a new stripeSeries with the given stripe size in an uninitialized state.
// When in an uninitialized state, reads and writes are not lock protected. After loading any
// initial data, a call to MarkInitialized() must be made before using the stripeSeries for
// ensuring proper function of stripeSeries.gc().
func newStripeSeries(stripeSize int) *stripeSeries {
	s := &stripeSeries{
		size:        stripeSize,
		series:      make([]map[chunks.HeadSeriesRef]*memSeries, stripeSize),
		hashes:      make([]seriesHashmap, stripeSize),
		exemplars:   make([]map[chunks.HeadSeriesRef]*exemplar.Exemplar, stripeSize),
		locks:       make([]stripeLock, stripeSize),
		initialized: atomic.NewBool(false),
	}
	for i := range s.series {
		s.series[i] = map[chunks.HeadSeriesRef]*memSeries{}
	}
	for i := range s.hashes {
		s.hashes[i] = seriesHashmap{}
	}
	for i := range s.exemplars {
		s.exemplars[i] = map[chunks.HeadSeriesRef]*exemplar.Exemplar{}
	}
	return s
}

// MarkInitialized marks the stripeSeries initialized, allowing usage of stripeSeries.gc(). Returns
// true if the stripeSeries was not initialized before, false otherwise.
func (s *stripeSeries) MarkInitialized() bool {
	return s.initialized.CompareAndSwap(false, true)
}

// RemoveInactiveSeries removes all series that have a lastTs of 0 while the stripeSeries is still in
// an uninitialized state. If the stripeSeries is already initialized, it returns 0 and false. Otherwise,
// it returns the number of series that were removed and true.
//
// The stripeSeries assumes that a chunks.HeadSeriesRef uniquely refers to a series in the stripeSeries.
// But in practice, a chunks.HeadSeriesRef can remain on a WAL even after it has been removed from the
// stripeSeries. If the series comes back before the original is removed from the WAL we are left with
// multiple chunks.HeadSeriesRef for the same series. If the WAL is reloaded in this state, we end up with a
// series leak. A call to stripeSeries.gc() is only capable of removing one instance of the chunks.HeadSeriesRef
// as it assumes there can only be one chunks.HeadSeriesRef for a series. The remaining chunks.HeadSeriesRefs are
// left in the stripeSeries and overtime can accumulate to consume a very large amount of memory.
func (s *stripeSeries) RemoveInactiveSeries() (int, bool) {
	if s.initialized.Load() {
		return 0, false
	}

	inactiveSeries := 0
	// Start with hashes first because it's easier to get to a series from the hash than a hash from a series.
	for _, hashSeries := range s.hashes {
		for hash, seriesForHash := range hashSeries {
			for _, series := range seriesForHash {
				if series.lastTs == 0 {
					hashSeries.Delete(hash, series.ref)
					inactiveSeries++

					// Get the seriesRef lock to delete the series from s.series.
					refLock := s.refLock(series.ref)
					delete(s.series[refLock], series.ref)
				}
			}
		}
	}

	for _, seriesRefs := range s.series {
		for head, series := range seriesRefs {
			if series.lastTs == 0 {
				delete(s.series[inactiveSeries], head)
				inactiveSeries++
			}
		}
	}

	return inactiveSeries, true
}

// gc garbage collects old chunks that are strictly before mint and removes
// series entirely that have no chunks left.
func (s *stripeSeries) gc(mint int64) map[chunks.HeadSeriesRef]struct{} {
	if !s.initialized.Load() {
		return nil
	}
	// NOTE(rfratto): GC will grab two locks, one for the hash and the other for
	// series. It's not valid for any other function to grab both locks,
	// otherwise a deadlock might occur when running GC in parallel with
	// appending.
	s.gcMut.Lock()
	defer s.gcMut.Unlock()

	deleted := map[chunks.HeadSeriesRef]struct{}{}
	for hashLock := 0; hashLock < s.size; hashLock++ {
		s.locks[hashLock].Lock()

		for hash, all := range s.hashes[hashLock] {
			for _, series := range all {
				series.Lock()

				// Any series that has received a write since mint is still alive.
				if series.lastTs >= mint {
					series.Unlock()
					continue
				}

				// The series is stale. We need to obtain a second lock for the
				// ref if it's different than the hash lock.
				refLock := int(s.refLock(series.ref))
				if hashLock != refLock {
					s.locks[refLock].Lock()
				}

				deleted[series.ref] = struct{}{}
				delete(s.series[refLock], series.ref)
				s.hashes[hashLock].Delete(hash, series.ref)

				// Since the series is gone, we'll also delete
				// the latest stored exemplar.
				delete(s.exemplars[refLock], series.ref)

				if hashLock != refLock {
					s.locks[refLock].Unlock()
				}
				series.Unlock()
			}
		}

		s.locks[hashLock].Unlock()
	}

	return deleted
}

func (s *stripeSeries) GetByID(id chunks.HeadSeriesRef) *memSeries {
	refLock := s.refLock(id)
	s.locks[refLock].RLock()
	defer s.locks[refLock].RUnlock()
	return s.series[refLock][id]
}

func (s *stripeSeries) GetByHash(hash uint64, lset labels.Labels) *memSeries {
	hashLock := s.hashLock(hash)

	s.locks[hashLock].RLock()
	defer s.locks[hashLock].RUnlock()
	return s.hashes[hashLock].Get(hash, lset)
}

func (s *stripeSeries) Set(hash uint64, series *memSeries) {
	var (
		hashLock = s.hashLock(hash)
		refLock  = s.refLock(series.ref)
	)

	// We can't hold both locks at once otherwise we might deadlock with a
	// simultaneous call to GC.
	//
	// We update s.series first because GC expects anything in s.hashes to
	// already exist in s.series.
	s.locks[refLock].Lock()
	s.series[refLock][series.ref] = series
	s.locks[refLock].Unlock()

	s.locks[hashLock].Lock()
	s.hashes[hashLock].Set(hash, series)
	s.locks[hashLock].Unlock()
}

func (s *stripeSeries) GetLatestExemplar(ref chunks.HeadSeriesRef) *exemplar.Exemplar {
	i := s.refLock(ref)

	s.locks[i].RLock()
	exemplar := s.exemplars[i][ref]
	s.locks[i].RUnlock()

	return exemplar
}

func (s *stripeSeries) SetLatestExemplar(ref chunks.HeadSeriesRef, exemplar *exemplar.Exemplar) {
	i := s.refLock(ref)

	// Make sure that's a valid series id and record its latest exemplar
	s.locks[i].Lock()
	if s.series[i][ref] != nil {
		s.exemplars[i][ref] = exemplar
	}
	s.locks[i].Unlock()
}

func (s *stripeSeries) iterator() *stripeSeriesIterator {
	return &stripeSeriesIterator{s}
}

func (s *stripeSeries) hashLock(hash uint64) uint64 {
	return hash & uint64(s.size-1)
}

func (s *stripeSeries) refLock(ref chunks.HeadSeriesRef) uint64 {
	return uint64(ref) & uint64(s.size-1)
}

// stripeSeriesIterator allows to iterate over series through a channel.
// The channel should always be completely consumed to not leak.
type stripeSeriesIterator struct {
	s *stripeSeries
}

func (it *stripeSeriesIterator) Channel() <-chan *memSeries {
	ret := make(chan *memSeries)

	go func() {
		for i := 0; i < it.s.size; i++ {
			it.s.locks[i].RLock()

			for _, series := range it.s.series[i] {
				series.Lock()

				j := int(it.s.hashLock(series.lset.Hash()))
				if i != j {
					it.s.locks[j].RLock()
				}

				ret <- series

				if i != j {
					it.s.locks[j].RUnlock()
				}
				series.Unlock()
			}

			it.s.locks[i].RUnlock()
		}

		close(ret)
	}()

	return ret
}
