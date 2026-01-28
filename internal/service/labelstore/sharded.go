package labelstore

import (
	"sync"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
)

// sharded wraps multiple single instances to distribute load and reduce mutex contention.
// Each shard is an independent labelstore with its own mutex.
type sharded struct {
	log       log.Logger
	shards    []*single
	numShards uint64
}

// GlobalRefID Encoding Strategy
//
// We encode the shard index in the upper 8 bits of globalRefID to enable routing:
//   Format: [8 bits: shard index][56 bits: per-shard ID]
//   Example: Shard 1, ID 42 → 0x0100000000000002A
//
// ID Space - No Meaningful Loss:
// - Per-shard: 2^56 = 72 quadrillion IDs (more than sufficient for any workload)
// - Max shards: 256 (2^8)
// - Even at 1 billion IDs/second, would take 2,283 years to exhaust one shard
//
// ID Generation is Controlled:
// - Each shard generates IDs sequentially (1, 2, 3, ...) under mutex protection
// - Starting from 0, incrementing by 1, guaranteed to stay within 56 bits
// - No external input can cause IDs to exceed the 56-bit space
//
// Routing Strategy:
// - GetOrAddGlobalRefID: Hash labels, route to shard (labels.Hash() % numShards)
// - GetLocalRefID: Extract shard from globalRefID upper bits, route to same shard
// - Same labelset always routes to same shard (deterministic)

const (
	// shardBits is the number of bits used to encode the shard index in globalRefID.
	// Upper 8 bits = 256 max shards, lower 56 bits = 2^56 IDs per shard.
	shardBits = 8
	// MaxShards is the maximum number of shards supported by the labelstore.
	MaxShards = 1 << shardBits // 256

	// shardMask extracts the shard index from globalRefID (upper 8 bits)
	shardMask = uint64(MaxShards-1) << (64 - shardBits)

	// localIDMask extracts the per-shard ID from globalRefID (lower 56 bits)
	localIDMask = ^shardMask
)

// newSharded creates a sharded labelstore with the specified number of shards.
// Each shard is an independent single instance with its own mutex.
//
// numShards must be between 1 and 256. Even with numShards=1, this implementation
// works correctly (useful for simplifying the codebase in the future).
func newSharded(l log.Logger, r prometheus.Registerer, numShards int) *sharded {
	shards := make([]*single, numShards)
	for i := 0; i < numShards; i++ {
		// Each shard uses the same registerer since metrics are aggregated by collect()
		shards[i] = newSingle(l, r)
	}

	return &sharded{
		log:       l,
		shards:    shards,
		numShards: uint64(numShards),
	}
}

// encodeGlobalRefID combines shard index and per-shard ID into a global ID.
// Format: [8 bits shard][56 bits local ID]
func (sh *sharded) encodeGlobalRefID(shardIdx uint64, localID uint64) uint64 {
	return (shardIdx << (64 - shardBits)) | (localID & localIDMask)
}

// decodeGlobalRefID extracts shard index and per-shard ID from global ID.
func (sh *sharded) decodeGlobalRefID(globalRefID uint64) (shardIdx uint64, localID uint64) {
	shardIdx = (globalRefID & shardMask) >> (64 - shardBits)
	localID = globalRefID & localIDMask
	return shardIdx, localID
}

// GetOrAddGlobalRefID routes to the appropriate shard based on label hash.
func (sh *sharded) GetOrAddGlobalRefID(l labels.Labels) uint64 {
	if l.IsEmpty() {
		return 0
	}

	labelHash := l.Hash()
	shardIdx := labelHash % sh.numShards
	shard := sh.shards[shardIdx]

	// Get the per-shard local ID
	localID := shard.GetOrAddGlobalRefID(l)

	// Encode shard index into the global ID
	return sh.encodeGlobalRefID(shardIdx, localID)
}

// GetLocalRefID extracts shard from globalRefID and routes to that shard.
func (sh *sharded) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
	shardIdx, localID := sh.decodeGlobalRefID(globalRefID)

	// Validate shard index to prevent panics from corrupted IDs
	if shardIdx >= sh.numShards {
		return 0
	}

	shard := sh.shards[shardIdx]
	return shard.GetLocalRefID(componentID, localID)
}

// AddLocalLink routes to the appropriate shard based on globalRefID.
func (sh *sharded) AddLocalLink(componentID string, globalRefID uint64, localRefID uint64) {
	shardIdx, localID := sh.decodeGlobalRefID(globalRefID)

	if shardIdx >= sh.numShards {
		return
	}

	shard := sh.shards[shardIdx]
	shard.AddLocalLink(componentID, localID, localRefID)
}

// ReplaceLocalLink routes to the appropriate shard based on globalRefID.
func (sh *sharded) ReplaceLocalLink(componentID string, globalRefID uint64, cachedLocalRef uint64, newLocalRef uint64) {
	shardIdx, localID := sh.decodeGlobalRefID(globalRefID)

	if shardIdx >= sh.numShards {
		return
	}

	shard := sh.shards[shardIdx]
	shard.ReplaceLocalLink(componentID, localID, cachedLocalRef, newLocalRef)
}

// TrackStaleness routes each tracker to its shard based on the globalRefID.
func (sh *sharded) TrackStaleness(ids []StalenessTracker) {
	if len(ids) == 0 {
		return
	}

	// Group trackers by shard to minimize lock acquisitions
	shardGroups := make(map[uint64][]StalenessTracker)
	for _, tracker := range ids {
		shardIdx, localID := sh.decodeGlobalRefID(tracker.GlobalRefID)
		if shardIdx >= sh.numShards {
			continue
		}

		// Rewrite the tracker with the per-shard local ID
		localTracker := StalenessTracker{
			GlobalRefID: localID,
			Value:       tracker.Value,
			Labels:      tracker.Labels,
		}
		shardGroups[shardIdx] = append(shardGroups[shardIdx], localTracker)
	}

	// Send grouped trackers to each shard
	for shardIdx, trackers := range shardGroups {
		sh.shards[shardIdx].TrackStaleness(trackers)
	}
}

// CheckAndRemoveStaleMarkers runs on all shards in parallel.
func (sh *sharded) CheckAndRemoveStaleMarkers() {
	var wg sync.WaitGroup
	wg.Add(len(sh.shards))

	for _, shard := range sh.shards {
		go func(shard *single) {
			defer wg.Done()
			shard.CheckAndRemoveStaleMarkers()
		}(shard)
	}

	wg.Wait()
}

// collect gathers and aggregates metrics from all shards.
// We must aggregate the counts instead of forwarding raw metrics to avoid duplicates.
func (sh *sharded) collect(m chan<- prometheus.Metric, totalIDsDesc *prometheus.Desc, idsInRemoteDesc *prometheus.Desc) {
	var (
		mu              sync.Mutex
		totalGlobalIDs  int
		remoteIDsByName = make(map[string]int)
		wg              sync.WaitGroup
	)

	wg.Add(len(sh.shards))

	for _, shard := range sh.shards {
		go func(shard *single) {
			defer wg.Done()

			// Lock shard and read its counts
			shard.mut.Lock()
			globalIDCount := len(shard.labelsHashToGlobal)
			mappingCounts := make(map[string]int)
			for name, rw := range shard.mappings {
				mappingCounts[name] = len(rw.globalToLocal)
			}
			shard.mut.Unlock()

			// Aggregate into totals
			mu.Lock()
			totalGlobalIDs += globalIDCount
			for name, count := range mappingCounts {
				remoteIDsByName[name] += count
			}
			mu.Unlock()
		}(shard)
	}

	wg.Wait()

	// Emit aggregated metrics
	m <- prometheus.MustNewConstMetric(totalIDsDesc, prometheus.GaugeValue, float64(totalGlobalIDs))
	for name, count := range remoteIDsByName {
		m <- prometheus.MustNewConstMetric(idsInRemoteDesc, prometheus.GaugeValue, float64(count), name)
	}
}
