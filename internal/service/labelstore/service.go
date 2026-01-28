package labelstore

import (
	"context"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	alloy_service "github.com/grafana/alloy/internal/service"
)

const ServiceName = "labelstore"

// Service is the labelstore service wrapper that delegates to either single or sharded implementation.
//
// Future simplification: sharded now supports numShards=1, so eventually Service could
// always use sharded, eliminating the single/sharded split. For now, we keep single for
// explicit backward compatibility and to make the refactoring reviewable.
type Service struct {
	log                 log.Logger
	single              *single
	sharded             *sharded
	totalIDs            *prometheus.Desc
	idsInRemoteWrapping *prometheus.Desc
}

type staleMarker struct {
	globalID        uint64
	lastMarkedStale time.Time
	labelHash       uint64
}

type remoteWriteMapping struct {
	RemoteWriteID string
	localToGlobal map[uint64]uint64
	globalToLocal map[uint64]uint64
}

type Arguments struct{}

var _ alloy_service.Service = (*Service)(nil)

// New creates a labelstore service with optional sharding support.
//
// Parameters:
//   - l: Logger for the service
//   - r: Prometheus registerer for metrics
//   - shards: Number of shards (1 = no sharding, 2-256 = sharded)
//
// With shards=1 (default), uses single-shard implementation with zero overhead.
// With shards>1, distributes load across N independent shards to reduce mutex contention.
func New(l log.Logger, r prometheus.Registerer, shards int) *Service {
	if l == nil {
		l = log.NewNopLogger()
	}
	if shards <= 0 {
		level.Warn(l).Log("msg", "shards <= 0, setting to 1", "shards", shards)
		shards = 1
	}
	if shards > MaxShards {
		level.Warn(l).Log("msg", "shards > MaxShards, setting to MaxShards", "shards", shards, "MaxShards", MaxShards)
		shards = MaxShards
	}

	s := &Service{
		log:                 l,
		totalIDs:            prometheus.NewDesc("alloy_labelstore_global_ids_count", "Total number of global ids.", nil, nil),
		idsInRemoteWrapping: prometheus.NewDesc("alloy_labelstore_remote_store_ids_count", "Total number of ids per remote write", []string{"remote_name"}, nil),
	}

	if shards == 1 {
		s.single = newSingle(l, r)
	} else {
		s.sharded = newSharded(l, r, shards)
	}

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
	if s.single != nil {
		s.single.collect(m, s.totalIDs, s.idsInRemoteWrapping)
		return
	}
	if s.sharded != nil {
		s.sharded.collect(m, s.totalIDs, s.idsInRemoteWrapping)
		return
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
			if s.single != nil {
				s.single.CheckAndRemoveStaleMarkers()
			} else if s.sharded != nil {
				s.sharded.CheckAndRemoveStaleMarkers()
			}
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

// LabelStore interface methods - delegate to single or sharded implementation

// GetOrAddGlobalRefID is used to create a global refid for a labelset
func (s *Service) GetOrAddGlobalRefID(l labels.Labels) uint64 {
	if s.single != nil {
		return s.single.GetOrAddGlobalRefID(l)
	}
	return s.sharded.GetOrAddGlobalRefID(l)
}

// GetLocalRefID returns the local refid for a component global combo, or 0 if not found
func (s *Service) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
	if s.single != nil {
		return s.single.GetLocalRefID(componentID, globalRefID)
	}
	return s.sharded.GetLocalRefID(componentID, globalRefID)
}

// AddLocalLink is called by a remote_write endpoint component to add mapping from local ref and global ref
func (s *Service) AddLocalLink(componentID string, globalRefID uint64, localRefID uint64) {
	if s.single != nil {
		s.single.AddLocalLink(componentID, globalRefID, localRefID)
		return
	}
	s.sharded.AddLocalLink(componentID, globalRefID, localRefID)
}

// ReplaceLocalLink updates an existing local to global mapping for a component.
func (s *Service) ReplaceLocalLink(componentID string, globalRefID uint64, cachedLocalRef uint64, newLocalRef uint64) {
	if s.single != nil {
		s.single.ReplaceLocalLink(componentID, globalRefID, cachedLocalRef, newLocalRef)
		return
	}
	s.sharded.ReplaceLocalLink(componentID, globalRefID, cachedLocalRef, newLocalRef)
}

// TrackStaleness adds a stale marker if NaN, then that reference will be removed on the next check.
func (s *Service) TrackStaleness(ids []StalenessTracker) {
	if s.single != nil {
		s.single.TrackStaleness(ids)
		return
	}
	s.sharded.TrackStaleness(ids)
}

// CheckAndRemoveStaleMarkers is called to garbage collect items that have grown stale over stale duration (10m)
func (s *Service) CheckAndRemoveStaleMarkers() {
	if s.single != nil {
		s.single.CheckAndRemoveStaleMarkers()
		return
	}
	s.sharded.CheckAndRemoveStaleMarkers()
}
