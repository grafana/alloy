package appenders

import (
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"
)

type MappingStore interface {
	GetMapping(uniqueRef storage.SeriesRef, lbls labels.Labels) []storage.SeriesRef
	CreateMapping(refResults []storage.SeriesRef, lbls labels.Labels) storage.SeriesRef
	UpdateMapping(uniqueRef storage.SeriesRef, refResults []storage.SeriesRef, lbls labels.Labels)
	GetCellForAppendedSeries() *Cell
	// OnCommit and OnRollback mark the end of an append batch; the appender calls
	// the one matching its own Commit or Rollback.
	OnCommit(ts int64, cell *Cell)
	OnRollback(ts int64, cell *Cell)
	// Clear drops all mappings and returns the new generation boundary.
	Clear() storage.SeriesRef
}

// EvictItem names one mapping to drop: its unique ref and the label hash of the
// series that owns it. The hash lets a drop skip a ref that now belongs to a
// different series.
type EvictItem struct {
	ref       storage.SeriesRef
	labelHash uint64
}

type seriesRefMapping struct {
	start    time.Time
	children []storage.Appender
	store    MappingStore

	uniqueRefCell *Cell

	// childRefs is reused for each append call to avoid allocations. This is safe because storage.Appender should never
	// have concurrent calls to Append methods.
	childRefs        []storage.SeriesRef
	writeLatency     prometheus.Histogram
	samplesForwarded prometheus.Counter
}

func NewSeriesRefMapping(children []storage.Appender, store MappingStore, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.Appender {
	uniqueRefCell := store.GetCellForAppendedSeries()

	return &seriesRefMapping{
		children:         children,
		store:            store,
		writeLatency:     writeLatency,
		samplesForwarded: samplesForwarded,

		uniqueRefCell: uniqueRefCell,
		childRefs:     make([]storage.SeriesRef, 0, len(children)),
	}
}

func (s *seriesRefMapping) SetOptions(opts *storage.AppendOptions) {
	for _, c := range s.children {
		c.SetOptions(opts)
	}
}

func (s *seriesRefMapping) Commit() error {
	defer s.recordLatency()

	s.store.OnCommit(time.Now().Unix(), s.uniqueRefCell)

	var multiErr error
	for _, c := range s.children {
		err := c.Commit()
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr
}

func (s *seriesRefMapping) Rollback() error {
	defer s.recordLatency()

	s.store.OnRollback(time.Now().Unix(), s.uniqueRefCell)

	var multiErr error
	for _, c := range s.children {
		err := c.Rollback()
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}
	return multiErr
}

func (s *seriesRefMapping) recordLatency() {
	if s.start.IsZero() {
		return
	}

	duration := time.Since(s.start)
	s.writeLatency.Observe(duration.Seconds())
}

func (s *seriesRefMapping) resetFields() {
	// Reset childRefs slice length to 0 for reuse
	s.childRefs = s.childRefs[:0]
}

func (s *seriesRefMapping) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	newRef, err := s.appendToChildren(ref, l, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		childRef, appendErr := appender.Append(ref, l, t, v)
		if appendErr == nil {
			s.samplesForwarded.Inc()
		}
		return childRef, appendErr
	})

	// A staleness marker is the series' last sample; queue its mapping so Commit
	// reclaims it promptly instead of waiting for the timestamp GC.
	if err == nil && value.IsStaleNaN(v) {
		s.uniqueRefCell.StaleRefs = append(s.uniqueRefCell.StaleRefs, EvictItem{ref: newRef, labelHash: l.Hash()})
	}

	return newRef, err
}

func (s *seriesRefMapping) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, l, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendExemplar(ref, l, e)
	})
}

func (s *seriesRefMapping) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, l, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendHistogram(ref, l, t, h, fh)
	})
}

func (s *seriesRefMapping) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, l, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendHistogramSTZeroSample(ref, l, t, st, h, fh)
	})
}

func (s *seriesRefMapping) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, l, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.UpdateMetadata(ref, l, m)
	})
}

func (s *seriesRefMapping) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, l, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendSTZeroSample(ref, l, t, st)
	})
}

type appenderFunc func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error)

func (s *seriesRefMapping) appendToChildren(ref storage.SeriesRef, lbls labels.Labels, af appenderFunc) (storage.SeriesRef, error) {
	defer s.resetFields()

	if s.start.IsZero() {
		s.start = time.Now()
	}

	// Check if the incoming ref has ref mappings
	existingChildRefs := s.store.GetMapping(ref, lbls)

	var appendErr error

	// Sanity check: if we have existing child refs, they must match the number of children
	if existingChildRefs != nil && len(existingChildRefs) == len(s.children) {
		s.uniqueRefCell.Refs = append(s.uniqueRefCell.Refs, ref)

		refUpdateRequired := false
		for childIndex, childRef := range existingChildRefs {
			newChildRef, err := af(s.children[childIndex], childRef)
			if err != nil {
				appendErr = multierror.Append(appendErr, err)
			}

			if newChildRef != childRef {
				refUpdateRequired = true
			}

			// Track refs in the local reuse buffer instead of mutating the shared mapping slice.
			s.childRefs = append(s.childRefs, newChildRef)
		}

		if appendErr != nil {
			return 0, appendErr
		}

		if refUpdateRequired {
			s.store.UpdateMapping(ref, s.childRefs, lbls)
		}

		return ref, nil
	}

	// No existing mapping, proceed with normal append to all children.
	var nonZeroCount int
	var nonZeroRef storage.SeriesRef
	for _, child := range s.children {
		childRef, err := af(child, ref)
		if err != nil {
			appendErr = multierror.Append(appendErr, err)
		}

		s.childRefs = append(s.childRefs, childRef)
		if childRef != 0 {
			nonZeroCount++
			nonZeroRef = childRef
		}
	}

	if appendErr != nil {
		return 0, appendErr
	}

	if nonZeroCount == 0 {
		// All children returned ref 0, so return the input ref
		return ref, nil
	}

	if nonZeroCount == 1 {
		// Only one child allocated a ref; return it directly — no mapping needed.
		return nonZeroRef, nil
	}

	uniqueRef := s.store.CreateMapping(s.childRefs, lbls)
	s.uniqueRefCell.Refs = append(s.uniqueRefCell.Refs, uniqueRef)
	return uniqueRef, nil
}

type uniqRefChildren struct {
	childRefs []storage.SeriesRef
	labelHash uint64
}

type SeriesRefMappingStore struct {
	// refMappingMu protects uniqueRefToChildRefs, labelHashToUniqueRef and nextUniqueRef
	refMappingMu sync.RWMutex
	// uniqueRefToChildRefs maps the unique ref to the expected child ref in order
	uniqueRefToChildRefs map[storage.SeriesRef]uniqRefChildren
	// labelHashToUniqueRef maps the label hash to unique ref.
	labelHashToUniqueRef map[uint64]storage.SeriesRef

	// nextUniqueRef is the next ref ID we will hand out
	nextUniqueRef storage.SeriesRef
	// firstRefOfCurrentGeneration is the first ref issued after the last Clear(). Any ref
	// below this value is from a previous generation and must be treated as unmapped.
	firstRefOfCurrentGeneration storage.SeriesRef

	// timestampTrackingMu protects uniqueRefTimestamps and cellPool
	timestampTrackingMu sync.Mutex
	// uniqueRefTimestamps maps unique refs to their last append timestamp
	uniqueRefTimestamps map[storage.SeriesRef]int64
	// cellPool pools the per-batch Cells handed to appenders for tracking unique refs.
	cellPool sync.Pool

	// Cleanup goroutine coordination (no lock required)
	startRefCleanup sync.Once
	cleanupStarted  atomic.Bool
	stopCleanup     chan struct{}
	cleanupStopped  chan struct{}

	// Metrics (safe for concurrent access, no lock required)
	activeMappings  prometheus.Gauge
	trackedRefs     prometheus.Gauge
	refsCleaned     prometheus.Counter
	staleNaNEvicted prometheus.Counter
	uniqueRefsTotal prometheus.Counter
}

func NewSeriesRefMappingStore(reg prometheus.Registerer) *SeriesRefMappingStore {
	activeMappings := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "alloy_fanout_mapping_store_mappings_total",
		Help: "Number of active unique ref mappings in the store.",
	})
	trackedRefs := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "alloy_fanout_mapping_store_tracked_refs_total",
		Help: "Number of refs being tracked for timestamp-based cleanup.",
	})
	refsCleaned := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alloy_fanout_mapping_store_refs_cleaned_total",
		Help: "Total number of stale refs cleaned up over time.",
	})
	staleNaNEvicted := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alloy_fanout_mapping_store_stale_nan_evictions_total",
		Help: "Total number of mappings evicted promptly on a staleness marker.",
	})
	uniqueRefsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alloy_fanout_mapping_store_unique_refs_created_total",
		Help: "Total number of unique refs created.",
	})

	if reg != nil {
		_ = reg.Register(activeMappings)
		_ = reg.Register(trackedRefs)
		_ = reg.Register(refsCleaned)
		_ = reg.Register(staleNaNEvicted)
		_ = reg.Register(uniqueRefsTotal)
	}

	return &SeriesRefMappingStore{
		uniqueRefToChildRefs: make(map[storage.SeriesRef]uniqRefChildren),
		nextUniqueRef:        1,
		uniqueRefTimestamps:  make(map[storage.SeriesRef]int64),
		labelHashToUniqueRef: make(map[uint64]storage.SeriesRef),
		cellPool: sync.Pool{
			New: func() any {
				return &Cell{Refs: make([]storage.SeriesRef, 0, 100)}
			},
		},
		stopCleanup:     make(chan struct{}),
		cleanupStopped:  make(chan struct{}),
		activeMappings:  activeMappings,
		trackedRefs:     trackedRefs,
		refsCleaned:     refsCleaned,
		staleNaNEvicted: staleNaNEvicted,
		uniqueRefsTotal: uniqueRefsTotal,
	}
}

type Cell struct {
	Refs []storage.SeriesRef
	// StaleRefs are mappings to evict on Commit: series that went stale this batch.
	StaleRefs []EvictItem
}

// GetMapping returns existing child ref results for the given unique ref if one exists.
//
// If the passed uniqueRef is zero, the method will attempt to find a mapping using passed labels.
// Returns nil if no mapping exists.
//
// The returned slice must be treated as read-only. Callers that need to change a mapping
// must provide an updated slice to UpdateMapping. Concurrent appenders may race to update
// the same mapping with different values, which is safe because stale mappings are
// self-correcting - using a stale ref will cause the child appender to return a new ref
// on the next append.
func (s *SeriesRefMappingStore) GetMapping(uniqueRef storage.SeriesRef, lbls labels.Labels) []storage.SeriesRef {
	s.refMappingMu.RLock()
	defer s.refMappingMu.RUnlock()

	if uniqueRef == 0 {
		// Some consumers don't memo the global ref. Try to lookup a ref by label hash.
		labelHash := lbls.Hash()
		gotRef, ok := s.labelHashToUniqueRef[labelHash]
		if !ok {
			return nil
		}

		uniqueRef = gotRef
	}

	// Refs below firstRefOfCurrentGeneration were issued before the last Clear() and are
	// no longer valid — the children they mapped to may have changed.
	if uniqueRef < s.firstRefOfCurrentGeneration {
		return nil
	}

	if mapping, ok := s.uniqueRefToChildRefs[uniqueRef]; ok {
		// Guard against numeric collisions with refs cached from a previous generation.
		if mapping.labelHash != lbls.Hash() {
			return nil
		}
		return mapping.childRefs
	}
	return nil
}

// CreateMapping creates a new unique ref mapping for the given child ref results.
func (s *SeriesRefMappingStore) CreateMapping(refResults []storage.SeriesRef, lbls labels.Labels) storage.SeriesRef {
	// Start cleanup goroutine on first mapping
	s.startRefCleanup.Do(func() {
		s.cleanupStarted.Store(true)
		go s.cleanupStaleRefs()
	})

	// Store a copy of the child ref results directly
	childRefSlice := make([]storage.SeriesRef, len(refResults))
	copy(childRefSlice, refResults)

	// Hash labels to for the fallback lookup table
	labelHash := lbls.Hash()

	s.refMappingMu.Lock()
	defer s.refMappingMu.Unlock()

	// Create a new unique ref
	uniqueRef := s.nextUniqueRef
	s.nextUniqueRef++

	s.labelHashToUniqueRef[labelHash] = uniqueRef
	s.uniqueRefToChildRefs[uniqueRef] = uniqRefChildren{
		childRefs: childRefSlice,
		labelHash: labelHash,
	}

	s.activeMappings.Inc()
	s.uniqueRefsTotal.Inc()

	return uniqueRef
}

func (s *SeriesRefMappingStore) UpdateMapping(uniqueRef storage.SeriesRef, refResults []storage.SeriesRef, lbls labels.Labels) {
	if uniqueRef == 0 {
		return
	}

	childRefSlice := make([]storage.SeriesRef, len(refResults))
	copy(childRefSlice, refResults)

	s.refMappingMu.Lock()
	defer s.refMappingMu.Unlock()

	newHash := lbls.Hash()
	prev, ok := s.uniqueRefToChildRefs[uniqueRef]
	if ok && prev.labelHash != newHash {
		delete(s.labelHashToUniqueRef, prev.labelHash)
		s.labelHashToUniqueRef[newHash] = uniqueRef
	}

	s.uniqueRefToChildRefs[uniqueRef] = uniqRefChildren{
		childRefs: childRefSlice,
		labelHash: lbls.Hash(),
	}
}

func (s *SeriesRefMappingStore) OnCommit(ts int64, cell *Cell) {
	evicted := s.evictStale(cell.StaleRefs)
	s.trackTimestamps(ts, cell, evicted)
}

func (s *SeriesRefMappingStore) OnRollback(ts int64, cell *Cell) {
	// nil: a rolled-back batch never committed downstream, so reclaim nothing.
	s.trackTimestamps(ts, cell, nil)
}

// evictStale drops the mappings named by items and returns the refs actually
// removed.
func (s *SeriesRefMappingStore) evictStale(items []EvictItem) []storage.SeriesRef {
	if len(items) == 0 {
		return nil
	}

	evicted := make([]storage.SeriesRef, 0, len(items))

	// Evict under refMappingMu alone; OnCommit tracks under timestampTrackingMu
	// afterward, so the two store locks are never held together.
	s.refMappingMu.Lock()
	for _, it := range items {
		// Skip refs from a past generation or recycled to another series. A ref
		// re-appended mid-drop self-corrects: the next append re-creates its mapping.
		if it.ref == 0 || it.ref < s.firstRefOfCurrentGeneration {
			continue
		}
		v, ok := s.uniqueRefToChildRefs[it.ref]
		if !ok || v.labelHash != it.labelHash {
			continue
		}
		delete(s.uniqueRefToChildRefs, it.ref)
		delete(s.labelHashToUniqueRef, v.labelHash)
		evicted = append(evicted, it.ref)
	}
	// Adjust activeMappings under refMappingMu: Clear zeroes it under the same lock,
	// so a Sub outside could race that reset and drive the gauge negative.
	if len(evicted) > 0 {
		s.activeMappings.Sub(float64(len(evicted)))
		s.staleNaNEvicted.Add(float64(len(evicted)))
	}
	s.refMappingMu.Unlock()

	return evicted
}

// trackTimestamps timestamps the batch's refs, drops the timestamps of any refs
// just evicted, and returns the cell to the pool.
func (s *SeriesRefMappingStore) trackTimestamps(ts int64, cell *Cell, evicted []storage.SeriesRef) {
	s.timestampTrackingMu.Lock()
	for _, r := range cell.Refs {
		s.uniqueRefTimestamps[r] = ts
	}
	for _, r := range evicted {
		delete(s.uniqueRefTimestamps, r)
	}
	s.trackedRefs.Set(float64(len(s.uniqueRefTimestamps)))
	s.timestampTrackingMu.Unlock()

	cell.Refs = cell.Refs[:0]
	cell.StaleRefs = cell.StaleRefs[:0]
	s.cellPool.Put(cell)
}

func (s *SeriesRefMappingStore) GetCellForAppendedSeries() *Cell {
	return s.cellPool.Get().(*Cell)
}

func (s *SeriesRefMappingStore) cleanupStaleRefs() {
	defer close(s.cleanupStopped)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.removeStaleRefs(time.Now().Add(-15 * time.Minute).Unix())
		case <-s.stopCleanup:
			return
		}
	}
}

// staleRefDeleteChunk bounds the mappings deleted per refMappingMu acquisition,
// so cleanup holds the write lock only briefly at a time rather than across the
// whole map.
const staleRefDeleteChunk = 8192

// removeStaleRefs removes every ref last appended before cutoff.
func (s *SeriesRefMappingStore) removeStaleRefs(cutoff int64) {
	// Scan for stale refs under timestampTrackingMu alone. The two store locks are
	// never held together, so appends reading GetMapping under refMappingMu don't
	// block on this O(N) scan.
	s.timestampTrackingMu.Lock()
	staleRefs := make([]storage.SeriesRef, 0)
	for ref, ts := range s.uniqueRefTimestamps {
		if ts < cutoff {
			staleRefs = append(staleRefs, ref)
			delete(s.uniqueRefTimestamps, ref)
		}
	}
	s.trackedRefs.Set(float64(len(s.uniqueRefTimestamps)))
	s.timestampTrackingMu.Unlock()

	if len(staleRefs) == 0 {
		return
	}

	// Delete the mappings in bounded chunks so each refMappingMu write stays short.
	// A ref re-appended between the scan and here may be dropped though it's live
	// again; that's self-correcting (the next append misses and re-creates it), and
	// the label-hash guard on GetMapping prevents misattribution.
	cleaned := 0
	for start := 0; start < len(staleRefs); start += staleRefDeleteChunk {
		end := min(start+staleRefDeleteChunk, len(staleRefs))

		s.refMappingMu.Lock()
		for _, ref := range staleRefs[start:end] {
			if v, ok := s.uniqueRefToChildRefs[ref]; ok {
				delete(s.labelHashToUniqueRef, v.labelHash)
				delete(s.uniqueRefToChildRefs, ref)
				cleaned++
			}
		}
		s.refMappingMu.Unlock()
	}

	if cleaned > 0 {
		s.refsCleaned.Add(float64(cleaned))
		s.activeMappings.Sub(float64(cleaned))
	}
}

// Clear will clear all internal mappings and stop the cleaner goroutine if it is running.
// It is safe to re-use the same instance after calling Clear.
// Returns the generation boundary; any ref below this value is stale.
func (s *SeriesRefMappingStore) Clear() storage.SeriesRef {
	// Stop the cleanup goroutine and wait for it to be stopped so we can
	// avoid a possible deadlock with cleanup that also holds both locks
	if s.cleanupStarted.Load() {
		select {
		case <-s.stopCleanup:
			// Already closed
		default:
			close(s.stopCleanup)
			<-s.cleanupStopped
		}
	}

	// Clearing all three maps must be atomic against in-flight appends, so hold both
	// locks. This is the only site that holds them together; any code path that does
	// must take timestampTrackingMu before refMappingMu to avoid deadlock.
	s.timestampTrackingMu.Lock()
	defer s.timestampTrackingMu.Unlock()

	s.refMappingMu.Lock()
	defer s.refMappingMu.Unlock()

	clear(s.uniqueRefToChildRefs)
	clear(s.uniqueRefTimestamps)
	clear(s.labelHashToUniqueRef)

	// reset the pool
	s.cellPool = sync.Pool{
		New: func() any {
			return &Cell{Refs: make([]storage.SeriesRef, 0, 100)}
		},
	}

	// NOTE: We do NOT reset nextUniqueRef here. Resetting it would cause ref collisions
	// with components like prometheus.scrape which will keep re-sending the same cached refs.
	// We continue incrementing to ensure all refs remain unique across the lifetime of the process.
	s.firstRefOfCurrentGeneration = s.nextUniqueRef

	// Reset metrics
	s.activeMappings.Set(0)
	s.trackedRefs.Set(0)

	// Reset channels and flags
	s.stopCleanup = make(chan struct{})
	s.cleanupStopped = make(chan struct{})
	s.startRefCleanup = sync.Once{}
	s.cleanupStarted.Store(false)

	return s.firstRefOfCurrentGeneration
}
