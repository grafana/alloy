package scrape

import (
	"testing"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
)

func TestClusteringExcludedLabels(t *testing.T) {
	first := clusteringTestTarget("__address__", "localhost:9090", "__param_sig", "first", "__meta_source", "one")
	second := clusteringTestTarget("__address__", "localhost:9090", "__param_sig", "second", "__meta_source", "two")
	otherAddress := clusteringTestTarget("__address__", "localhost:9091", "__param_sig", "first")
	otherPath := clusteringTestTarget("__address__", "localhost:9090", "__metrics_path__", "/other", "__param_sig", "first")
	otherParam := clusteringTestTarget("__address__", "localhost:9090", "__param_database", "other", "__param_sig", "first")
	addressKey := clusteringTestKey(clusteringTestTarget("__address__", "localhost:9090"))

	for _, tc := range []struct {
		name           string
		excludedLabels []string
		targets        []discovery.Target
		ownershipKeys  []shard.Key
	}{
		{
			name:          "default hashes are unchanged",
			targets:       []discovery.Target{first, second},
			ownershipKeys: []shard.Key{clusteringTestKey(first), clusteringTestKey(second)},
		},
		{
			name:           "empty exclusions preserve hashes",
			excludedLabels: []string{},
			targets:        []discovery.Target{first, second},
			ownershipKeys:  []shard.Key{clusteringTestKey(first), clusteringTestKey(second)},
		},
		{
			name:           "missing labels have no effect",
			excludedLabels: []string{"missing"},
			targets:        []discovery.Target{first, second},
			ownershipKeys:  []shard.Key{clusteringTestKey(first), clusteringTestKey(second)},
		},
		{
			name:           "patterns are not expanded",
			excludedLabels: []string{"__param_*"},
			targets:        []discovery.Target{first, second},
			ownershipKeys:  []shard.Key{clusteringTestKey(first), clusteringTestKey(second)},
		},
		{
			name:           "shared ownership retains distinct targets",
			excludedLabels: []string{"__param_sig"},
			targets:        []discovery.Target{first, second, otherAddress, otherPath, otherParam},
			ownershipKeys: []shard.Key{
				addressKey, addressKey,
				clusteringTestKey(clusteringTestTarget("__address__", "localhost:9091")),
				clusteringTestKey(clusteringTestTarget("__address__", "localhost:9090", "__metrics_path__", "/other")),
				clusteringTestKey(clusteringTestTarget("__address__", "localhost:9090", "__param_database", "other")),
			},
		},
		{
			name:           "duplicate exclusions have no effect",
			excludedLabels: []string{"__param_sig", "__param_sig", "missing"},
			targets:        []discovery.Target{first, second},
			ownershipKeys:  []shard.Key{addressKey, addressKey},
		},
		{
			name:           "empty ownership key retains all targets",
			excludedLabels: []string{"__address__", "__param_sig"},
			targets:        []discovery.Target{first, second, otherAddress},
			ownershipKeys:  []shard.Key{clusteringTestKey(discovery.EmptyTarget), clusteringTestKey(discovery.EmptyTarget), clusteringTestKey(discovery.EmptyTarget)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, invert := range []bool{false, true} {
				lookup := make(map[shard.Key][]peer.Peer)
				for i, key := range tc.ownershipKeys {
					owner := peer2
					if (i%2 == 0) != invert {
						owner = peer1Self
					}
					lookup[key] = []peer.Peer{owner}
				}
				var lookedUp []shard.Key
				c := &fakeCluster{
					peers: []peer.Peer{peer1Self, peer2, peer3}, lookupMap: lookup,
					onLookup: func(key shard.Key) { lookedUp = append(lookedUp, key) },
				}
				dt := discovery.NewDistributedTargets(true, c, tc.targets, ClusteringConfig{Enabled: true, ExcludedLabels: tc.excludedLabels}.targetHashing())
				require.Equal(t, tc.ownershipKeys, lookedUp)
				require.Equal(t, len(tc.targets), dt.TargetCount())
				expected := make([]discovery.Target, 0)
				for i, key := range tc.ownershipKeys {
					if lookup[key][0].Self {
						expected = append(expected, tc.targets[i])
					}
				}
				require.Equal(t, expected, dt.LocalTargets())
			}
		})
	}
}

func clusteringTestTarget(kv ...string) discovery.Target {
	labels := make(map[string]string, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		labels[kv[i]] = kv[i+1]
	}
	return discovery.NewTargetFromMap(labels)
}

func clusteringTestKey(target discovery.Target) shard.Key {
	return shard.Key(target.NonMetaLabelsHash())
}
