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
	if !clusteringEnabled || cluster == nil {
		cluster = disabledCluster{}
	}

	localCap := len(allTargets) + 1
	if peerCount := len(cluster.Peers()); peerCount != 0 {
		localCap = (len(allTargets) + 1) / peerCount
	}

	localTargets := make([]Target, 0, localCap)
	localTargetKeys := make([]shard.Key, 0, localCap)
	remoteTargetKeys := make(map[shard.Key]struct{}, len(allTargets)-localCap)

	for _, tgt := range allTargets {
		targetKey := keyFor(tgt)
		peers, err := cluster.Lookup(targetKey, 1, shard.OpReadWrite)
		belongsToLocal := err != nil || len(peers) == 0 || peers[0].Self

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

// MovedToRemoteInstance returns targets that have been moved from this local node to another node, given the provided
// previous targets distribution. If a target has simply disappeared, it will not be returned. Previous targets
// distribution can be nil, in which case no targets will be returned.
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
	return shard.Key(tgt.NonMetaLabels().Hash())
}

type disabledCluster struct{}

var _ cluster.Cluster = disabledCluster{}

func (l disabledCluster) Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error) {
	return nil, nil
}

func (l disabledCluster) Peers() []peer.Peer {
	return nil
}
