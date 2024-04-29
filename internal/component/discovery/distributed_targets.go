package discovery

import (
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/ckit/shard"
)

// DistributedTargets uses the node's Lookup method to distribute discovery
// targets when a component runs in a cluster.
type DistributedTargets struct {
	useClustering bool
	cluster       cluster.Cluster
	targets       []Target
}

// NewDistributedTargets creates the abstraction that allows components to
// dynamically shard targets between components.
func NewDistributedTargets(e bool, n cluster.Cluster, t []Target) DistributedTargets {
	return DistributedTargets{e, n, t}
}

// Get distributes discovery targets a clustered environment.
//
// If a cluster size is 1, then all targets will be returned.
func (t *DistributedTargets) Get() []Target {
	// TODO(@tpaschalis): Make this into a single code-path to simplify logic.
	if !t.useClustering || t.cluster == nil {
		return t.targets
	}

	peerCount := len(t.cluster.Peers())
	resCap := (len(t.targets) + 1)
	if peerCount != 0 {
		resCap = (len(t.targets) + 1) / peerCount
	}

	res := make([]Target, 0, resCap)

	for _, tgt := range t.targets {
		//TODO(thampiotr): we use non meta labels and hash the string of it, instead of using hash of the labels
		// 				   check if this can be improved by using Hash()
		peers, err := t.cluster.Lookup(shard.StringKey(tgt.NonMetaLabels().String()), 1, shard.OpReadWrite)
		if err != nil {
			// This can only fail in case we ask for more owners than the
			// available peers. This will never happen, but in any case we fall
			// back to owning the target ourselves.
			res = append(res, tgt)
		}
		if len(peers) == 0 || peers[0].Self {
			res = append(res, tgt)
		}
	}

	return res
}
