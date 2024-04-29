package discovery

import (
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/ckit/shard"
)

// DistributedTargets uses the node's Lookup method to distribute discovery
// targets when a component runs in a cluster.
type DistributedTargets struct {
	localTargets     []Target
	remoteTargetKeys map[shard.Key]struct{}
}

// NewDistributedTargets creates the abstraction that allows components to
// dynamically shard targets between components.
func NewDistributedTargets(clusteringEnabled bool, cluster cluster.Cluster, allTargets []Target) DistributedTargets {
	// TODO(@tpaschalis): Make this into a single code-path to simplify logic.
	if !clusteringEnabled || cluster == nil {
		return DistributedTargets{
			localTargets:     allTargets,
			remoteTargetKeys: map[shard.Key]struct{}{},
		}
	}

	peerCount := len(cluster.Peers())
	localCap := len(allTargets) + 1
	if peerCount != 0 {
		localCap = (len(allTargets) + 1) / peerCount
	}

	localTargets := make([]Target, 0, localCap)
	remoteTargetKeys := make(map[shard.Key]struct{})

	for _, tgt := range allTargets {
		targetKey := keyFor(tgt)
		peers, err := cluster.Lookup(targetKey, 1, shard.OpReadWrite)
		belongsToLocal := err != nil || len(peers) == 0 || peers[0].Self

		if belongsToLocal {
			localTargets = append(localTargets, tgt)
		} else {
			remoteTargetKeys[targetKey] = struct{}{}
		}
	}

	return DistributedTargets{localTargets: localTargets, remoteTargetKeys: remoteTargetKeys}
}

// LocalTargets returns the targets that belong to the local cluster node.
func (dt *DistributedTargets) LocalTargets() []Target {
	return dt.localTargets
}

// MovedAway returns targets that have been moved from this local node to another node, given the provided
// previous targets distribution. Previous targets distribution can be nil, in which case no targets moved away.
func (dt *DistributedTargets) MovedAway(prev *DistributedTargets) []Target {
	if prev == nil {
		return nil
	}
	var movedAwayTargets []Target
	for _, previousLocal := range prev.localTargets {
		key := keyFor(previousLocal)
		if _, exist := dt.remoteTargetKeys[key]; exist {
			return append(movedAwayTargets, previousLocal)
		}
	}
	return movedAwayTargets
}

func keyFor(tgt Target) shard.Key {
	//TODO(thampiotr): we use non meta labels and hash the string of it, instead of using hash of the labels
	// 				   check if this can be improved by using Hash()
	return shard.StringKey(tgt.NonMetaLabels().String())
}
