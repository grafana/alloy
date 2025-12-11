package labelstore

import (
	"context"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	alloy_service "github.com/grafana/alloy/internal/service"
)

const ServiceName = "labelstore"

type Service struct {
	log                 log.Logger
	mut                 sync.RWMutex
	globalRefID         uint64
	mappings            map[string]*remoteWriteMapping
	labelsHashToGlobal  map[uint64]uint64
	staleGlobals        map[uint64]*staleMarker
	totalIDs            *prometheus.Desc
	idsInRemoteWrapping *prometheus.Desc
	lastStaleCheck      prometheus.Gauge
}
type staleMarker struct {
	globalID        uint64
	lastMarkedStale time.Time
	labelHash       uint64
}

type Arguments struct{}

var _ alloy_service.Service = (*Service)(nil)

func New(l log.Logger, r prometheus.Registerer) *Service {
	if l == nil {
		l = log.NewNopLogger()
	}
	s := &Service{
		log:                 l,
		globalRefID:         0,
		mappings:            make(map[string]*remoteWriteMapping),
		labelsHashToGlobal:  make(map[uint64]uint64),
		staleGlobals:        make(map[uint64]*staleMarker),
		totalIDs:            prometheus.NewDesc("alloy_labelstore_global_ids_count", "Total number of global ids.", nil, nil),
		idsInRemoteWrapping: prometheus.NewDesc("alloy_labelstore_remote_store_ids_count", "Total number of ids per remote write", []string{"remote_name"}, nil),
		lastStaleCheck: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "alloy_labelstore_last_stale_check_timestamp",
			Help: "Last time stale check was ran expressed in unix timestamp.",
		}),
	}
	_ = r.Register(s.lastStaleCheck)
	_ = r.Register(s)
	return s
}

// Definition returns the Definition of the Service.
// Definition must always return the same value across all
// calls.
func (s *Service) Definition() alloy_service.Definition {
	return alloy_service.Definition{
		Name:       ServiceName,
		ConfigType: Arguments{},
		DependsOn:  nil,
		Stability:  featuregate.StabilityGenerallyAvailable,
	}
}

func (s *Service) Describe(m chan<- *prometheus.Desc) {
	m <- s.totalIDs
	m <- s.idsInRemoteWrapping
}

func (s *Service) Collect(m chan<- prometheus.Metric) {
	s.mut.Lock()
	defer s.mut.Unlock()

	m <- prometheus.MustNewConstMetric(s.totalIDs, prometheus.GaugeValue, float64(len(s.labelsHashToGlobal)))
	for name, rw := range s.mappings {
		m <- prometheus.MustNewConstMetric(s.idsInRemoteWrapping, prometheus.GaugeValue, float64(len(rw.globalToLocal)), name)
	}
}

// Run starts a Service. Run must block until the provided
// context is canceled. Returning an error should be treated
// as a fatal error for the Service.
func (s *Service) Run(ctx context.Context, host alloy_service.Host) error {
	staleCheck := time.NewTicker(10 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-staleCheck.C:
			s.CheckAndRemoveStaleMarkers()
		}
	}
}

// Update updates a Service at runtime. Update is never
// called if [Definition.ConfigType] is nil. newConfig will
// be the same type as ConfigType; if ConfigType is a
// pointer to a type, newConfig will be a pointer to the
// same type.
//
// Update will be called once before Run, and may be called
// while Run is active.
func (s *Service) Update(_ any) error {
	return nil
}

// Data returns the Data associated with a Service. Data
// must always return the same value across multiple calls,
// as callers are expected to be able to cache the result.
//
// Data may be invoked before Run.
func (s *Service) Data() any {
	return s
}

// AddLocalLink is called by a remote_write endpoint component to add mapping from local ref and global ref
func (s *Service) AddLocalLink(componentID string, globalRefID uint64, localRefID uint64) {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.addLocalLink(componentID, globalRefID, localRefID)
}

func (s *Service) addLocalLink(componentID string, globalRefID uint64, localRefID uint64) {
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
func (s *Service) ReplaceLocalLink(componentID string, globalRefID uint64, cachedLocalRef uint64, newLocalRef uint64) {
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
func (s *Service) GetOrAddGlobalRefID(l labels.Labels) uint64 {
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
func (s *Service) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
	s.mut.RLock()
	defer s.mut.RUnlock()

	m, found := s.mappings[componentID]
	if !found {
		return 0
	}
	local := m.globalToLocal[globalRefID]
	return local
}

func (s *Service) TrackStaleness(ids []StalenessTracker) {
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

// staleDuration determines how long we should wait after a stale value is received to GC that value
var staleDuration = time.Minute * 10

// CheckAndRemoveStaleMarkers is called to garbage collect and items that have grown stale over stale duration (10m)
func (s *Service) CheckAndRemoveStaleMarkers() {
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

func (rw *remoteWriteMapping) deleteStaleIDs(globalID uint64) {
	localID, found := rw.globalToLocal[globalID]
	if !found {
		return
	}
	delete(rw.globalToLocal, globalID)
	delete(rw.localToGlobal, localID)
}

// remoteWriteMapping maps a remote_write to a set of global ids
type remoteWriteMapping struct {
	RemoteWriteID string
	localToGlobal map[uint64]uint64
	globalToLocal map[uint64]uint64
}
