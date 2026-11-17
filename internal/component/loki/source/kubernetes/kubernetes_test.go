package kubernetes

import (
	"testing"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/source/kubernetes/kubetail"
	"github.com/grafana/alloy/internal/service/cluster"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	targets    = [
		{"__address__" = "localhost:9090", "foo" = "bar"},
		{"__address__" = "localhost:8080", "foo" = "buzz"},
	]
    forward_to = []
	client {
		api_server = "localhost:9091"
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestBadAlloyConfig(t *testing.T) {
	var exampleAlloyConfig = `
	targets    = [
		{"__address__" = "localhost:9090", "foo" = "bar"},
		{"__address__" = "localhost:8080", "foo" = "buzz"},
	]
    forward_to = []
	client {
		api_server = "localhost:9091"
		bearer_token = "token"
		bearer_token_file = "/path/to/file.token"
	}
`

	// Make sure the squashed HTTPClientConfig Validate function is being utilized correctly
	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.ErrorContains(t, err, "at most one of basic_auth, authorization, oauth2, bearer_token & bearer_token_file must be configured")
}

func TestClusteringDuplicateAddress(t *testing.T) {
	// Different scrape endpoints for the same container identify the same logs.
	distTargets := discovery.NewDistributedTargets(
		true,
		cluster.Mock(),
		[]discovery.Target{
			discovery.NewTargetFromMap(map[string]string{
				"__address__": "localhost:9090",
				"container":   "alloy",
				"pod":         "grafana-k8s-monitoring-alloy-0",
				"job":         "integrations/alloy",
				"namespace":   "default",
			}),
			discovery.NewTargetFromMap(map[string]string{
				"__address__": "localhost:8080",
				"container":   "alloy",
				"pod":         "grafana-k8s-monitoring-alloy-0",
				"job":         "integrations/alloy",
				"namespace":   "default",
			}),
		},
		discovery.IdentityLabels(kubetail.ClusteringLabels),
	)
	require.True(t, distTargets.TargetCount() == 1)
}

func TestClusteringIdentityAndOwnership(t *testing.T) {
	target := func(address, uid string) discovery.Target {
		return discovery.NewTargetFromMap(map[string]string{
			"__address__":                          address,
			"__meta_kubernetes_namespace":          "default",
			"__meta_kubernetes_pod_name":           "app",
			"__meta_kubernetes_pod_container_name": "app",
			"__meta_kubernetes_pod_uid":            uid,
		})
	}
	first := target("localhost:9090", "uid-one")
	duplicate := target("localhost:8080", "uid-one")
	other := target("localhost:9090", "uid-two")
	firstKey := shard.Key(first.SpecificLabelsHash(kubetail.ClusteringLabels))
	otherKey := shard.Key(other.SpecificLabelsHash(kubetail.ClusteringLabels))
	require.NotEqual(t, firstKey, otherKey)
	var keys []shard.Key
	c := lookupCluster{Cluster: cluster.Mock(), lookup: func(key shard.Key) ([]peer.Peer, error) {
		keys = append(keys, key)
		return []peer.Peer{{Self: key == firstKey}}, nil
	}}
	dt := discovery.NewDistributedTargets(true, c, []discovery.Target{first, duplicate, other}, discovery.IdentityLabels(kubetail.ClusteringLabels))
	require.Equal(t, []shard.Key{firstKey, otherKey}, keys)
	require.Equal(t, []discovery.Target{first}, dt.LocalTargets())
	require.Equal(t, 2, dt.TargetCount())
}

type lookupCluster struct {
	cluster.Cluster
	lookup func(shard.Key) ([]peer.Peer, error)
}

func (c lookupCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	return c.lookup(key)
}
