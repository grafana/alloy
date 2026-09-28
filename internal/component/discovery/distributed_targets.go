package discovery

import (
	"slices"
	"strings"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/service/cluster"
)

// DistributedTargets uses the node's Lookup method to distribute discovery
// targets when a component runs in a cluster.
type DistributedTargets struct {
	localTargets        []Target               // Targets assigned to this instance.
	remoteOwnershipKeys map[shard.Key]struct{} // Ownership hashes assigned to other instances.
	targetCount         int                    // Number of distinct target identities across all instances.
	ownershipHash       func(Target) uint64    // Current ownership hash function, also used to detect handoffs.
}

// TargetHashing separates target identity from cluster ownership.
// Functions must be deterministic, and equal identities must have equal ownership
// hashes. Prefer the presets below when selecting or excluding labels.
type TargetHashing struct {
	Identity  func(Target) uint64 // Deduplication hash; defaults to NonMetaLabelsHash.
	Ownership func(Target) uint64 // Cluster ownership hash; defaults to the identity hash.
}

// IdentityLabels uses selected labels for both identity and ownership, including
// any selected __meta_* labels. An empty list uses the default non-meta labels.
func IdentityLabels(labels []string) TargetHashing {
	if len(labels) == 0 {
		return TargetHashing{}
	}
	labels = slices.Clone(labels)
	return TargetHashing{Identity: func(target Target) uint64 {
		return target.SpecificLabelsHash(labels)
	}}
}

// ExcludeOwnershipLabels excludes exact label names from the non-meta ownership
// hash, while preserving the default identity hash and original targets.
func ExcludeOwnershipLabels(labels []string) TargetHashing {
	if len(labels) == 0 {
		return TargetHashing{}
	}
	labels = slices.Clone(labels)
	return TargetHashing{Ownership: func(target Target) uint64 {
		return target.HashLabelsWithPredicate(func(name string) bool {
			return !strings.HasPrefix(name, model.MetaLabelPrefix) && !slices.Contains(labels, name)
		})
	}}
}

// NewDistributedTargets distributes targets using the supplied hashing rules.
// A zero TargetHashing uses all non-meta labels for both identity and ownership.
func NewDistributedTargets(clusteringEnabled bool, cluster cluster.Cluster, allTargets []Target, hashing TargetHashing) *DistributedTargets {
	if hashing.Identity == nil {
		hashing.Identity = Target.NonMetaLabelsHash
	}
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
	remoteOwnershipKeys := make(map[shard.Key]struct{}, len(allTargets)-localCap)

	// Need to handle duplicate entries.
	unique := make(map[shard.Key]struct{})
	for _, tgt := range allTargets {
		targetKey := shard.Key(hashing.Identity(tgt))

		// check if we have already seen this target
		if _, ok := unique[targetKey]; ok {
			continue
		}
		unique[targetKey] = struct{}{}
		ownershipKey := targetKey
		if hashing.Ownership != nil {
			ownershipKey = shard.Key(hashing.Ownership(tgt))
		}

		// Determine if target belongs locally. Make sure it doesn't if cluster not ready.
		belongsToLocal := false
		if cluster.Ready() {
			peers, err := cluster.Lookup(ownershipKey, 1, shard.OpReadWrite)
			belongsToLocal = err != nil || len(peers) == 0 || peers[0].Self
		}

		if belongsToLocal {
			localTargets = append(localTargets, tgt)
		} else {
			remoteOwnershipKeys[ownershipKey] = struct{}{}
		}
	}

	if hashing.Ownership == nil {
		hashing.Ownership = hashing.Identity
	}
	return &DistributedTargets{
		localTargets:        localTargets,
		remoteOwnershipKeys: remoteOwnershipKeys,
		targetCount:         len(unique),
		ownershipHash:       hashing.Ownership,
	}
}

// LocalTargets returns the targets that belong to the local cluster node.
func (dt *DistributedTargets) LocalTargets() []Target {
	return dt.localTargets
}

func (dt *DistributedTargets) TargetCount() int {
	return dt.targetCount
}

// MovedToRemoteInstance returns previous local targets whose ownership group is
// now remote. This includes targets that disappeared from a group that moved,
// preferring to suppress staleness over marking a potentially active series stale.
func (dt *DistributedTargets) MovedToRemoteInstance(prev *DistributedTargets) []Target {
	if prev == nil || len(dt.remoteOwnershipKeys) == 0 {
		return nil
	}
	var movedAwayTargets []Target
	for _, target := range prev.localTargets {
		// Use the current rules so a configuration reload can also move targets.
		key := shard.Key(dt.ownershipHash(target))
		if _, exist := dt.remoteOwnershipKeys[key]; exist {
			movedAwayTargets = append(movedAwayTargets, target)
		}
	}
	return movedAwayTargets
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

func (l disabledCluster) Enabled() bool {
	return false
}
