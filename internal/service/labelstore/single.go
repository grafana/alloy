package labelstore

import (
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// single is the original single-shard labelstore implementation.
// It holds all the core logic with a single mutex protecting all data structures.
type single struct {
	log                 log.Logger
	mut                 sync.Mutex
	globalRefID         uint64
	mappings            map[string]*remoteWriteMapping
	labelsHashToGlobal  map[uint64]uint64
	staleGlobals        map[uint64]*staleMarker
	lastStaleCheck      prometheus.Gauge
}

func newSingle(l log.Logger, r prometheus.Registerer) *single {
	if l == nil {
		l = log.NewNopLogger()
	}
	s := &single{
		log:                 l,
		globalRefID:         0,
		mappings:            make(map[string]*remoteWriteMapping),
		labelsHashToGlobal:  make(map[uint64]uint64),
		staleGlobals:        make(map[uint64]*staleMarker),
		lastStaleCheck: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_labelstore_last_stale_check_timestamp",
			Help: "Last time stale check was ran expressed in unix timestamp.",
		}),
	}
	_ = r.Register(s.lastStaleCheck)
	return s
}

// AddLocalLink is called by a remote_write endpoint component to add mapping from local ref and global ref
func (s *single) AddLocalLink(componentID string, globalRefID uint64, localRefID uint64) {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.addLocalLink(componentID, globalRefID, localRefID)
}

func (s *single) addLocalLink(componentID string, globalRefID uint64, localRefID uint64) {
	// If the mapping doesn't exist then we need to create it
	m, found := s.mappings[componentID]
	if !found {
		m = &remoteWriteMapping{
			RemoteWriteID: componentID,
			localToGlobal: make(map[uint64]uint64),
			globalToLocal: make(map[uint64]uint64),
		}
		s.mappings[componentID] = m
	}

	m.localToGlobal[localRefID] = globalRefID
	m.globalToLocal[globalRefID] = localRefID
}

// ReplaceLocalLink updates an existing local to global mapping for a component.
func (s *single) ReplaceLocalLink(componentID string, globalRefID uint64, cachedLocalRef uint64, newLocalRef uint64) {
	s.mut.Lock()
	defer s.mut.Unlock()

	m, found := s.mappings[componentID]
	// If we don't have a mapping yet there's nothing to replace
	if !found {
		s.addLocalLink(componentID, globalRefID, newLocalRef)
		return
	}

	// Delete the old mapping
	delete(m.localToGlobal, cachedLocalRef)
	// Add the new mapping
	m.localToGlobal[newLocalRef] = globalRefID
	m.globalToLocal[globalRefID] = newLocalRef
}

// GetOrAddGlobalRefID is used to create a global refid for a labelset
func (s *single) GetOrAddGlobalRefID(l labels.Labels) uint64 {
	// Guard against bad input.
	if l.IsEmpty() {
		return 0
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	labelHash := l.Hash()
	globalID, found := s.labelsHashToGlobal[labelHash]
	if found {
		return globalID
	}
	s.globalRefID++
	s.labelsHashToGlobal[labelHash] = s.globalRefID
	return s.globalRefID
}

// GetLocalRefID returns the local refid for a component global combo, or 0 if not found
func (s *single) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
	s.mut.Lock()
	defer s.mut.Unlock()

	m, found := s.mappings[componentID]
	if !found {
		return 0
	}
	local := m.globalToLocal[globalRefID]
	return local
}

func (s *single) TrackStaleness(ids []StalenessTracker) {
	var (
		toAdd    = make([]*staleMarker, 0)
		toRemove = make([]uint64, 0)
		now      = time.Now()
	)

	for _, id := range ids {
		if value.IsStaleNaN(id.Value) {
			toAdd = append(toAdd, &staleMarker{
				globalID:        id.GlobalRefID,
				lastMarkedStale: now,
				labelHash:       id.Labels.Hash(),
			})
		} else {
			toRemove = append(toRemove, id.GlobalRefID)
		}
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	for _, marker := range toAdd {
		s.staleGlobals[marker.globalID] = marker
	}
	for _, id := range toRemove {
		delete(s.staleGlobals, id)
	}
}

// CheckAndRemoveStaleMarkers is called to garbage collect and items that have grown stale over stale duration (10m)
func (s *single) CheckAndRemoveStaleMarkers() {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.lastStaleCheck.Set(float64(time.Now().Unix()))
	level.Debug(s.log).Log("msg", "labelstore removing stale markers")
	curr := time.Now()
	idsToBeGCed := make([]*staleMarker, 0)
	for _, stale := range s.staleGlobals {
		// If the difference between now and the last time the stale was marked doesn't exceed stale then let it stay
		if curr.Sub(stale.lastMarkedStale) < staleDuration {
			continue
		}
		idsToBeGCed = append(idsToBeGCed, stale)
	}

	level.Debug(s.log).Log("msg", "number of ids to remove", "count", len(idsToBeGCed))

	for _, marker := range idsToBeGCed {
		delete(s.staleGlobals, marker.globalID)
		delete(s.labelsHashToGlobal, marker.labelHash)
		// Delete our mapping keys
		for _, mapping := range s.mappings {
			mapping.deleteStaleIDs(marker.globalID)
		}
	}
}

// collect gathers prometheus metrics for this single-shard implementation
func (s *single) collect(m chan<- prometheus.Metric, totalIDsDesc *prometheus.Desc, idsInRemoteDesc *prometheus.Desc) {
	s.mut.Lock()
	defer s.mut.Unlock()

	m <- prometheus.MustNewConstMetric(totalIDsDesc, prometheus.GaugeValue, float64(len(s.labelsHashToGlobal)))
	for name, rw := range s.mappings {
		m <- prometheus.MustNewConstMetric(idsInRemoteDesc, prometheus.GaugeValue, float64(len(rw.globalToLocal)), name)
	}
}

func (rw *remoteWriteMapping) deleteStaleIDs(globalID uint64) {
	localID, found := rw.globalToLocal[globalID]
	if !found {
		return
	}
	delete(rw.globalToLocal, globalID)
	delete(rw.localToGlobal, localID)
}

// staleDuration determines how long we should wait after a stale value is received to GC that value
var staleDuration = time.Minute * 10

