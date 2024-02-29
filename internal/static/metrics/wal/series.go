package wal

// This code is copied from
// prometheus/prometheus@7c2de14b0bd74303c2ca6f932b71d4585a29ca75, with only
// minor changes for metric names.

import (
	"sync"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"
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
	size      int
	series    []map[chunks.HeadSeriesRef]*memSeries
	hashes    []seriesHashmap
	exemplars []map[chunks.HeadSeriesRef]*exemplar.Exemplar
	locks     []stripeLock

	gcMut sync.Mutex
}

type stripeLock struct {
	sync.RWMutex
	// Padding to avoid multiple locks being on the same cache line.
	_ [40]byte
}

func newStripeSeries(stripeSize int) *stripeSeries {
	s := &stripeSeries{
		size:      stripeSize,
		series:    make([]map[chunks.HeadSeriesRef]*memSeries, stripeSize),
		hashes:    make([]seriesHashmap, stripeSize),
		exemplars: make([]map[chunks.HeadSeriesRef]*exemplar.Exemplar, stripeSize),
		locks:     make([]stripeLock, stripeSize),
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

// gc garbage collects old chunks that are strictly before mint and removes
// series entirely that have no chunks left.
func (s *stripeSeries) gc(mint int64) map[chunks.HeadSeriesRef]struct{} {
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
				refLock := int(series.ref) & (s.size - 1)
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
	refLock := uint64(id) & uint64(s.size-1)
	s.locks[refLock].RLock()
	defer s.locks[refLock].RUnlock()
	return s.series[refLock][id]
}

func (s *stripeSeries) GetByHash(hash uint64, lset labels.Labels) *memSeries {
	hashLock := hash & uint64(s.size-1)

	s.locks[hashLock].RLock()
	defer s.locks[hashLock].RUnlock()
	return s.hashes[hashLock].Get(hash, lset)
}

func (s *stripeSeries) Set(hash uint64, series *memSeries) {
	var (
		hashLock = hash & uint64(s.size-1)
		refLock  = uint64(series.ref) & uint64(s.size-1)
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
	i := uint64(ref) & uint64(s.size-1)

	s.locks[i].RLock()
	exemplar := s.exemplars[i][ref]
	s.locks[i].RUnlock()

	return exemplar
}

func (s *stripeSeries) SetLatestExemplar(ref chunks.HeadSeriesRef, exemplar *exemplar.Exemplar) {
	i := uint64(ref) & uint64(s.size-1)

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

				j := int(series.lset.Hash()) & (it.s.size - 1)
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
