package kubernetes

import (
	"testing"

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
	// Since loki.source.kubernetes looks up by pod name, if we dont use the special NewDistributedTargetsWithCustomLabels
	// then we can pull logs multiple times if the address is reused for the port. This works fine for scraping since those are different
	// endpoints, but from a log perspective they are the same logs.
	distTargets := discovery.NewDistributedTargetsWithCustomLabels(
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
		kubetail.ClusteringLabels,
	)
	require.True(t, distTargets.TargetCount() == 1)
}
