package discovery

import (
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"

	"github.com/grafana/alloy/internal/service/cluster"
)

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

		// Determine if target belongs locally. Make sure it doesn't if cluster not ready.
		belongsToLocal := false
		if cluster.Ready() {
			peers, err := cluster.Lookup(targetKey, 1, shard.OpReadWrite)
			belongsToLocal = err != nil || len(peers) == 0 || peers[0].Self
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
