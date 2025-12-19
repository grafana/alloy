package appenders

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

type mappingStore interface {
	GetMapping(uniqueRef storage.SeriesRef) []storage.SeriesRef
	CreateMapping(refResults []storage.SeriesRef) storage.SeriesRef
	UpdateMapping(uniqueRef storage.SeriesRef, refResults []storage.SeriesRef)
	TrackAppendedSeries(ts int64, refs []storage.SeriesRef)
	GetSliceForAppendedSeries() []storage.SeriesRef
}

type seriesRefMapping struct {
	start    time.Time
	children []storage.Appender
	store    mappingStore

	appendedUniqueRefs []storage.SeriesRef

	// childRefs is reused for each append call to avoid allocations. This is safe because storage.Appender should never
	// have concurrent calls to Append methods.
	childRefs        []storage.SeriesRef
	writeLatency     prometheus.Histogram
	samplesForwarded prometheus.Counter
}

func NewSeriesRefMapping(children []storage.Appender, store mappingStore, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.Appender {
	appendedUniqueRefs := store.GetSliceForAppendedSeries()

	return &seriesRefMapping{
		children:         children,
		store:            store,
		writeLatency:     writeLatency,
		samplesForwarded: samplesForwarded,

		appendedUniqueRefs: appendedUniqueRefs,
		childRefs:          make([]storage.SeriesRef, 0, len(children)),
	}
}

func (s *seriesRefMapping) SetOptions(opts *storage.AppendOptions) {
	for _, c := range s.children {
		c.SetOptions(opts)
	}
}

func (s *seriesRefMapping) Commit() error {
	s.store.TrackAppendedSeries(time.Now().Unix(), s.appendedUniqueRefs)

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
	// We still track rolled back series so we can properly
	// clean up any series that was appended
	s.store.TrackAppendedSeries(time.Now().Unix(), s.appendedUniqueRefs)

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
	return s.appendToChildren(ref, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		newRef, err := appender.Append(ref, l, t, v)
		if err == nil {
			s.samplesForwarded.Inc()
		}
		return newRef, err
	})
}

func (s *seriesRefMapping) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendExemplar(ref, l, e)
	})
}

func (s *seriesRefMapping) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendHistogram(ref, l, t, h, fh)
	})
}

func (s *seriesRefMapping) AppendHistogramCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendHistogramCTZeroSample(ref, l, t, ct, h, fh)
	})
}

func (s *seriesRefMapping) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.UpdateMetadata(ref, l, m)
	})
}

func (s *seriesRefMapping) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	return s.appendToChildren(ref, func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error) {
		return appender.AppendCTZeroSample(ref, l, t, ct)
	})
}

type appenderFunc func(appender storage.Appender, ref storage.SeriesRef) (storage.SeriesRef, error)

func (s *seriesRefMapping) appendToChildren(ref storage.SeriesRef, af appenderFunc) (storage.SeriesRef, error) {
	defer s.resetFields()

	if s.start.IsZero() {
		s.start = time.Now()
	}

	// Check if the incoming ref has ref mappings
	existingChildRefs := s.store.GetMapping(ref)

	var appendErr error

	// Sanity check: if we have existing child refs, they must match the number of children
	if existingChildRefs != nil && len(existingChildRefs) == len(s.children) {
		s.appendedUniqueRefs = append(s.appendedUniqueRefs, ref)

		refUpdateRequired := false
		for childIndex, childRef := range existingChildRefs {
			newChildRef, err := af(s.children[childIndex], childRef)
			if err != nil {
				appendErr = multierror.Append(appendErr, err)
			}

			if newChildRef != childRef {
				// Child ref changed, need to update mapping
				existingChildRefs[childIndex] = newChildRef
				refUpdateRequired = true
			}
		}

		if appendErr != nil {
			return 0, appendErr
		}

		if refUpdateRequired {
			s.store.UpdateMapping(ref, existingChildRefs)
		}

		return ref, nil
	}

	// No existing mapping, proceed with normal append to all children
	var firstNonZeroRef storage.SeriesRef
	var nonZeroCount int

	// Note: there's another optimization where we could use the returned ref if all the non zero refs
	//  are the same value. This isn't safe as we will mix downstream refs with unique refs which could
	//  collide.We could start at max unit64 for unique refs and go backwards lessening the chance of
	// 	collisions. But it's rather dangerous for an unlikely edge case. If two components are returning
	// 	the same ref it's two remote_write components which should probably be merged in to one.
	for _, child := range s.children {
		childRef, err := af(child, ref)
		if err != nil {
			appendErr = multierror.Append(appendErr, err)
		}

		s.childRefs = append(s.childRefs, childRef)
		if childRef != 0 {
			if firstNonZeroRef == 0 {
				firstNonZeroRef = childRef
			}
			nonZeroCount++
		}
	}

	if appendErr != nil {
		return 0, appendErr
	}

	if nonZeroCount == 0 {
		// All children returned ref 0, so return the input ref
		return ref, nil
	}

	// Only one child returned a non-zero ref, use that
	if nonZeroCount == 1 {
		return firstNonZeroRef, nil
	}

	// We got different refs back and need to create a new mapping
	uniqueRef := s.store.CreateMapping(s.childRefs)
	s.appendedUniqueRefs = append(s.appendedUniqueRefs, uniqueRef)
	return uniqueRef, nil
}

type SeriesRefMappingStore struct {
	// refMappingMu protects uniqueRefToChildRefs and nextUniqueRef
	refMappingMu sync.RWMutex
	// uniqueRefToChildRefs maps the unique ref to the expected child ref in order
	uniqueRefToChildRefs map[storage.SeriesRef][]storage.SeriesRef
	// nextUniqueRef is the next ref ID we will hand out
	nextUniqueRef storage.SeriesRef

	// timestampTrackingMu protects uniqueRefTimestamps and appendedUniqueRefsSlicePool
	timestampTrackingMu sync.Mutex
	// uniqueRefTimestamps maps unique refs to their last append timestamp
	uniqueRefTimestamps map[storage.SeriesRef]int64
	// appendedUniqueRefsSlicePool is used to pool slices of SeriesRefs used for tracking appendedUniqueRefs
	appendedUniqueRefsSlicePool sync.Pool

	// Cleanup goroutine coordination (no lock required)
	startRefCleanup sync.Once
	cleanupStarted  atomic.Bool
	stopCleanup     chan struct{}
	cleanupStopped  chan struct{}

	// Metrics (safe for concurrent access, no lock required)
	activeMappings  prometheus.Gauge
	trackedRefs     prometheus.Gauge
	refsCleaned     prometheus.Counter
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
	uniqueRefsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "alloy_fanout_mapping_store_unique_refs_created_total",
		Help: "Total number of unique refs created.",
	})

	if reg != nil {
		reg.Register(activeMappings)
		reg.Register(trackedRefs)
		reg.Register(refsCleaned)
		reg.Register(uniqueRefsTotal)
	}

	return &SeriesRefMappingStore{
		uniqueRefToChildRefs: make(map[storage.SeriesRef][]storage.SeriesRef),
		nextUniqueRef:        1,
		uniqueRefTimestamps:  make(map[storage.SeriesRef]int64),
		appendedUniqueRefsSlicePool: sync.Pool{
			New: func() any {
				return make([]storage.SeriesRef, 0, 100)
			},
		},
		stopCleanup:     make(chan struct{}),
		cleanupStopped:  make(chan struct{}),
		activeMappings:  activeMappings,
		trackedRefs:     trackedRefs,
		refsCleaned:     refsCleaned,
		uniqueRefsTotal: uniqueRefsTotal,
	}
}

// GetMapping returns existing child ref results for the given unique ref if one exists.
// Returns nil if no mapping exists.
//
// The returned slice may be modified by the caller, but UpdateMapping must be called
// afterwards to persist changes. Note that concurrent appenders may race to update the
// same mapping with different values, which is safe because stale mappings are self-correcting -
// using a stale ref will cause the child appender to return a new ref on the next append.
func (s *SeriesRefMappingStore) GetMapping(uniqueRef storage.SeriesRef) []storage.SeriesRef {
	if uniqueRef == 0 {
		return nil
	}

	s.refMappingMu.RLock()
	defer s.refMappingMu.RUnlock()

	if childRefs, ok := s.uniqueRefToChildRefs[uniqueRef]; ok {
		return childRefs
	}
	return nil
}

// CreateMapping creates a new unique ref mapping for the given child ref results.
func (s *SeriesRefMappingStore) CreateMapping(refResults []storage.SeriesRef) storage.SeriesRef {
	// Start cleanup goroutine on first mapping
	s.startRefCleanup.Do(func() {
		s.cleanupStarted.Store(true)
		go s.cleanupStaleRefs()
	})

	// Store a copy of the child ref results directly
	childRefSlice := make([]storage.SeriesRef, len(refResults))
	copy(childRefSlice, refResults)

	s.refMappingMu.Lock()
	defer s.refMappingMu.Unlock()

	// Create a new unique ref
	uniqueRef := s.nextUniqueRef
	s.nextUniqueRef++

	s.uniqueRefToChildRefs[uniqueRef] = childRefSlice

	s.activeMappings.Inc()
	s.uniqueRefsTotal.Inc()

	return uniqueRef
}

func (s *SeriesRefMappingStore) UpdateMapping(uniqueRef storage.SeriesRef, refResults []storage.SeriesRef) {
	if uniqueRef == 0 {
		return
	}

	childRefSlice := make([]storage.SeriesRef, len(refResults))
	copy(childRefSlice, refResults)

	s.refMappingMu.Lock()
	defer s.refMappingMu.Unlock()

	s.uniqueRefToChildRefs[uniqueRef] = childRefSlice
}

func (s *SeriesRefMappingStore) TrackAppendedSeries(ts int64, refs []storage.SeriesRef) {
	s.timestampTrackingMu.Lock()
	defer s.timestampTrackingMu.Unlock()

	for _, r := range refs {
		s.uniqueRefTimestamps[r] = ts
	}

	s.trackedRefs.Set(float64(len(s.uniqueRefTimestamps)))

	refs = refs[:0]
	s.appendedUniqueRefsSlicePool.Put(refs)
}

func (s *SeriesRefMappingStore) GetSliceForAppendedSeries() []storage.SeriesRef {
	return s.appendedUniqueRefsSlicePool.Get().([]storage.SeriesRef)
}

func (s *SeriesRefMappingStore) cleanupStaleRefs() {
	defer close(s.cleanupStopped)

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cutoffTime := time.Now().Add(-15 * time.Minute).Unix()

			// Hold both locks to prevent race condition where a ref could be
			// appended after we delete it from appendedUniqueRefs but before
			// we delete it from uniqueRefToChildRefs
			s.timestampTrackingMu.Lock()
			s.refMappingMu.Lock()

			staleRefs := make([]storage.SeriesRef, 0)
			for ref, ts := range s.uniqueRefTimestamps {
				if ts < cutoffTime {
					staleRefs = append(staleRefs, ref)
					delete(s.uniqueRefTimestamps, ref)
					delete(s.uniqueRefToChildRefs, ref)
				}
			}

			// Update metrics
			if len(staleRefs) > 0 {
				s.refsCleaned.Add(float64(len(staleRefs)))
				s.activeMappings.Sub(float64(len(staleRefs)))
				s.trackedRefs.Set(float64(len(s.uniqueRefTimestamps)))
			}

			s.refMappingMu.Unlock()
			s.timestampTrackingMu.Unlock()

		case <-s.stopCleanup:
			return
		}
	}
}

// Clear will clear all internal mappings and stop the cleaner goroutine if it is running.
// It is safe to re-use the same instance after calling Clear.
func (s *SeriesRefMappingStore) Clear() {
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

	// We need to hold both locks to do this safely and we do it in the same order as
	// cleanupStaleRefs. We stopped and waited for the background worker that calls it
	// to finish but some extra safety won't hurt.
	s.timestampTrackingMu.Lock()
	defer s.timestampTrackingMu.Unlock()

	s.refMappingMu.Lock()
	defer s.refMappingMu.Unlock()

	clear(s.uniqueRefToChildRefs)
	clear(s.uniqueRefTimestamps)

	// reset the pool
	s.appendedUniqueRefsSlicePool = sync.Pool{
		New: func() any {
			return make([]storage.SeriesRef, 0, 100)
		},
	}

	// NOTE: We do NOT reset nextUniqueRef here. Resetting it would cause ref collisions
	// with components like prometheus.scrape which will keep re-sending the same cached refs.
	// We continue incrementing to ensure all refs remain unique across the lifetime of the process.

	// Reset metrics
	s.activeMappings.Set(0)
	s.trackedRefs.Set(0)

	// Reset channels and flags
	s.stopCleanup = make(chan struct{})
	s.cleanupStopped = make(chan struct{})
	s.startRefCleanup = sync.Once{}
	s.cleanupStarted.Store(false)
}
