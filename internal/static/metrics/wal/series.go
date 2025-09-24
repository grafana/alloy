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

// Upstream prometheus implementation https://github.com/prometheus/prometheus/blob/main/tsdb/agent/series.go

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

// seriesHashmap lets stores a memSeries by its label set, via a 64-bit hash.
// There is one map for the common case where the hash value is unique, and a
// second map for the case that two series have the same hash value.
// Each series is in only one of the maps. Its methods require the hash to be submitted
// with the label set to avoid re-computing hash throughout the code.
type seriesHashmap struct {
	unique    map[uint64]*memSeries
	conflicts map[uint64][]*memSeries
}

func (m *seriesHashmap) Get(hash uint64, lset labels.Labels) *memSeries {
	if s, found := m.unique[hash]; found {
		if labels.Equal(s.lset, lset) {
			return s
		}
	}
	for _, s := range m.conflicts[hash] {
		if labels.Equal(s.lset, lset) {
			return s
		}
	}
	return nil
}

func (m *seriesHashmap) Set(hash uint64, s *memSeries) {
	if existing, found := m.unique[hash]; !found || labels.Equal(existing.lset, s.lset) {
		m.unique[hash] = s
		return
	}
	if m.conflicts == nil {
		m.conflicts = make(map[uint64][]*memSeries)
	}
	seriesSet := m.conflicts[hash]
	for i, prev := range seriesSet {
		if labels.Equal(prev.lset, s.lset) {
			seriesSet[i] = s
			return
		}
	}
	m.conflicts[hash] = append(seriesSet, s)
}

func (m *seriesHashmap) Delete(hash uint64, ref chunks.HeadSeriesRef) {
	var rem []*memSeries
	unique, found := m.unique[hash]
	switch {
	case !found: // Supplied hash is not stored.
		return
	case unique.ref == ref:
		conflicts := m.conflicts[hash]
		if len(conflicts) == 0 { // Exactly one series with this hash was stored
			delete(m.unique, hash)
			return
		}
		m.unique[hash] = conflicts[0] // First remaining series goes in 'unique'.
		rem = conflicts[1:]           // Keep the rest.
	default: // The series to delete is somewhere in 'conflicts'. Keep all the ones that don't match.
		for _, s := range m.conflicts[hash] {
			if s.ref != ref {
				rem = append(rem, s)
			}
		}
	}
	if len(rem) == 0 {
		delete(m.conflicts, hash)
	} else {
		m.conflicts[hash] = rem
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
		s.hashes[i] = seriesHashmap{
			unique:    map[uint64]*memSeries{},
			conflicts: nil, // Initialized on demand in set().
		}
	}
	for i := range s.exemplars {
		s.exemplars[i] = map[chunks.HeadSeriesRef]*exemplar.Exemplar{}
	}
	return s
}

// gc garbage collects old series that have not received a sample after mint
// and will fully delete them.
func (s *stripeSeries) gc(mint int64) map[chunks.HeadSeriesRef]struct{} {
	// NOTE(rfratto): GC will grab two locks, one for the hash and the other for
	// series. It's not valid for any other function to grab both locks,
	// otherwise a deadlock might occur when running GC in parallel with
	// appending.
	s.gcMut.Lock()
	defer s.gcMut.Unlock()

	deleted := map[chunks.HeadSeriesRef]struct{}{}

	// For one series, check if it is stale and delete it and the latest exemplar if so.
	check := func(hashLock int, hash uint64, series *memSeries) {
		series.Lock()

		// Any series that has received a write since mint is still alive.
		if series.lastTs >= mint {
			series.Unlock()
			return
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

	for hashLock := 0; hashLock < s.size; hashLock++ {
		s.locks[hashLock].Lock()

		for hash, all := range s.hashes[hashLock].conflicts {
			for _, series := range all {
				check(hashLock, hash, series)
			}
		}
		for hash, series := range s.hashes[hashLock].unique {
			check(hashLock, hash, series)
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

// GetOrSet returns the existing series for the given hash and label set, or sets it if it does not exist.
// It returns the series and a boolean indicating whether it was newly created.
func (s *stripeSeries) GetOrSet(hash uint64, lset labels.Labels, series *memSeries) (*memSeries, bool) {
	hashLock := s.hashLock(hash)

	s.locks[hashLock].Lock()
	// If it already exists in hashes, return it.
	if prev := s.hashes[hashLock].Get(hash, lset); prev != nil {
		s.locks[hashLock].Unlock()
		return prev, false
	}
	s.hashes[hashLock].Set(hash, series)
	s.locks[hashLock].Unlock()

	refLock := s.refLock(series.ref)

	s.locks[refLock].Lock()
	s.series[refLock][series.ref] = series
	s.locks[refLock].Unlock()

	return series, true
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
