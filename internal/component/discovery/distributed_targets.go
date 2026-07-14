package discovery

import (
	"strconv"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"

	"github.com/grafana/alloy/internal/service/cluster"
)

// ClusteringKeyMetaLabel carries a target's clustering key (its
// NonMetaLabelsHash) as a per-target meta-label. In allocator mode the scrape
// component stamps it onto the targets it scrapes so the scrape-time series
// counter can attribute scraped series back to the exact distribution key the
// allocator uses — recomputing the hash from Prometheus' discovered labels would
// not match, since Prometheus adds synthetic non-meta labels (__scheme__,
// __scrape_interval__, …). As a __meta_ label it is excluded from
// NonMetaLabelsHash and dropped from stored series, so it has no cardinality
// impact and doesn't change the key it carries.
const ClusteringKeyMetaLabel = "__meta_alloy_clustering_key__"

// StampClusteringKeys returns copies of targets, each stamped with
// ClusteringKeyMetaLabel set to that target's clustering key.
func StampClusteringKeys(targets []Target) []Target {
	out := make([]Target, len(targets))
	for i, t := range targets {
		key := strconv.FormatUint(t.NonMetaLabelsHash(), 10)
		out[i] = t.withOwnLabel(ClusteringKeyMetaLabel, key)
	}
	return out
}

// targetsToEntries converts discovered targets into the neutral, JSON-friendly
// TargetEntry form the cluster allocator stores and serves. The key is the
// target's clustering key (NonMetaLabelsHash) and Labels is the full label set
// (including __meta_* labels, which downstream relabeling needs). Duplicate keys
// collapse to one entry, matching how DistributedTargets de-duplicates.
func targetsToEntries(targets []Target) []cluster.TargetEntry {
	seen := make(map[uint64]struct{}, len(targets))
	out := make([]cluster.TargetEntry, 0, len(targets))
	for _, t := range targets {
		key := t.NonMetaLabelsHash()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, cluster.TargetEntry{Key: key, Labels: t.AsMap()})
	}
	return out
}

// entriesToTargets reconstructs scrape-able targets from allocator entries. The
// reconstructed target's NonMetaLabelsHash equals the original key because the
// full label set is preserved and the hash is deterministic over it.
func entriesToTargets(entries []cluster.TargetEntry) []Target {
	out := make([]Target, 0, len(entries))
	for _, e := range entries {
		out = append(out, NewTargetFromMap(e.Labels))
	}
	return out
}

// DistributedTargets uses the node's Lookup method to distribute discovery
// targets when a component runs in a cluster.
type DistributedTargets struct {
	localTargets []Target
	// localTargetKeys is used to cache the key hash computation. Improves time performance by ~20%.
	localTargetKeys  []shard.Key
	remoteTargetKeys map[shard.Key]struct{}
}

// NewDistributedTargets creates the abstraction that allows components to
// dynamically shard targets between components.
func NewDistributedTargets(clusteringEnabled bool, cluster cluster.Cluster, allTargets []Target) *DistributedTargets {
	return NewDistributedTargetsWithCustomLabels(clusteringEnabled, cluster, allTargets, nil)
}

// NewDistributedTargetsWithCustomLabels creates the abstraction that allows components to
// dynamically shard targets between components. Passing in labels will limit the sharding to only use those labels for computing the hash key.
// Passing in nil or empty array means look at all labels.
func NewDistributedTargetsWithCustomLabels(clusteringEnabled bool, cluster cluster.Cluster, allTargets []Target, labels []string) *DistributedTargets {
	if !clusteringEnabled || cluster == nil {
		cluster = disabledCluster{}
	}

	var localCap int
	if !cluster.Ready() {
		localCap = 0 // cluster not ready - won't take any traffic locally
	} else if peerCount := len(cluster.Peers()); peerCount != 0 {
		localCap = (len(allTargets) + 1) / peerCount // if we have peers - calculate expected capacity
	} else {
		localCap = len(allTargets) // cluster ready but no peers? fall back to all traffic locally
	}

	localTargets := make([]Target, 0, localCap)
	localTargetKeys := make([]shard.Key, 0, localCap)
	remoteTargetKeys := make(map[shard.Key]struct{}, len(allTargets)-localCap)

	// Need to handle duplicate entries.
	unique := make(map[shard.Key]struct{})
	for _, tgt := range allTargets {
		var targetKey shard.Key
		// If we have no custom labels check all non-meta labels.
		if len(labels) == 0 {
			targetKey = keyFor(tgt)
		} else {
			targetKey = keyForLabels(tgt, labels)
		}

		// check if we have already seen this target
		if _, ok := unique[targetKey]; ok {
			continue
		}
		unique[targetKey] = struct{}{}

		// Determine if target belongs locally using the count-based hash ring.
		// Don't take traffic if the cluster is not ready.
		belongsToLocal := false
		if cluster.Ready() {
			belongsToLocal = ownedByCountBasedLookup(cluster, targetKey)
		}

		if belongsToLocal {
			localTargets = append(localTargets, tgt)
			localTargetKeys = append(localTargetKeys, targetKey)
		} else {
			remoteTargetKeys[targetKey] = struct{}{}
		}
	}

	return &DistributedTargets{
		localTargets:     localTargets,
		localTargetKeys:  localTargetKeys,
		remoteTargetKeys: remoteTargetKeys,
	}
}

// LocalTargets returns the targets that belong to the local cluster node.
func (dt *DistributedTargets) LocalTargets() []Target {
	return dt.localTargets
}

func (dt *DistributedTargets) TargetCount() int {
	return len(dt.localTargetKeys) + len(dt.remoteTargetKeys)
}

// MovedToRemoteInstance returns the set of local targets from prev
// that are no longer local in dt, indicating an active target has moved.
// Only targets which exist in both prev and dt are returned. If prev
// contains an empty list of targets, no targets are returned.
func (dt *DistributedTargets) MovedToRemoteInstance(prev *DistributedTargets) []Target {
	if prev == nil {
		return nil
	}
	var movedAwayTargets []Target
	for i := 0; i < len(prev.localTargets); i++ {
		key := prev.localTargetKeys[i]
		if _, exist := dt.remoteTargetKeys[key]; exist {
			movedAwayTargets = append(movedAwayTargets, prev.localTargets[i])
		}
	}
	return movedAwayTargets
}

// ownedByCountBasedLookup reports whether the local node owns the key under the
// default count-based hash ring. Ownership only applies once the cluster is
// ready; before that the node takes no traffic.
func ownedByCountBasedLookup(cluster cluster.Cluster, key shard.Key) bool {
	if !cluster.Ready() {
		return false
	}
	peers, err := cluster.Lookup(key, 1, shard.OpReadWrite)
	return err != nil || len(peers) == 0 || peers[0].Self
}

func keyFor(tgt Target) shard.Key {
	return shard.Key(tgt.NonMetaLabelsHash())
}

func keyForLabels(tgt Target, lbls []string) shard.Key {
	return shard.Key(tgt.SpecificLabelsHash(lbls))
}

type disabledCluster struct{}

var _ cluster.Cluster = disabledCluster{}

func (l disabledCluster) Lookup(_ shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	return nil, nil
}

func (l disabledCluster) Peers() []peer.Peer {
	return nil
}

func (l disabledCluster) Ready() bool {
	return true
}

func (l disabledCluster) AllocatorEnabled() bool { return false }

func (l disabledCluster) IsAllocatorLeader() bool { return false }

func (l disabledCluster) RegisterDiscoveredTargets(_ string, _ []cluster.TargetEntry) {}

func (l disabledCluster) AssignedTargets(_ string) ([]cluster.TargetEntry, error) { return nil, nil }

func (l disabledCluster) ReportWeights(_ map[uint64]uint64) {}
