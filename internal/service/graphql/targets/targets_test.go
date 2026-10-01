package targets_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/alloy/internal/service/graphql/targets"
)

func TestTargetsExtractsDirectDiscoveryTargets(t *testing.T) {
	info := &component.Info{Exports: directTargetExports{
		Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"job": "api"})},
	}}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, []model.LabelPair{{Name: "job", Value: "api"}}, got[0].Labels)
}

func TestTargetsExtractsNestedDiscoveryTargets(t *testing.T) {
	info := &component.Info{Exports: nestedTargetExports{
		Nested: nestedTargets{Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{"instance": "localhost:9090"})}},
	}}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Equal(t, "instance", got[0].Labels[0].Name)
}

func TestTargetsReturnsEmptyForComponentsWithoutTargets(t *testing.T) {
	info := &component.Info{Exports: struct{ Value string }{Value: "no targets"}}

	got := targets.FromComponent(info)

	require.Empty(t, got)
}

func TestTargetLabelsExcludeMetaLabelsAndHashesDiffer(t *testing.T) {
	info := &component.Info{Exports: directTargetExports{
		Targets: []discovery.Target{discovery.NewTargetFromMap(map[string]string{
			"job":                       "api",
			"__meta_kubernetes_pod":     "api-0",
			"__address__":               "localhost:9090",
			"__meta_kubernetes_node":    "node-1",
			"__param_module":            "default",
			"__scrape_interval__":       "15s",
			"__meta_kubernetes_cluster": "prod",
		})},
	}}

	got := targets.FromComponent(info)

	require.Len(t, got, 1)
	require.Contains(t, got[0].Labels, model.LabelPair{Name: "__meta_kubernetes_pod", Value: "api-0"})
	require.NotContains(t, got[0].NonMetaLabels, model.LabelPair{Name: "__meta_kubernetes_pod", Value: "api-0"})
	require.Contains(t, got[0].NonMetaLabels, model.LabelPair{Name: "job", Value: "api"})
	require.NotEmpty(t, got[0].Hash)
	require.NotEmpty(t, got[0].NonMetaHash)
	require.NotEqual(t, got[0].Hash, got[0].NonMetaHash)
}

type directTargetExports struct {
	Targets []discovery.Target
}

type nestedTargetExports struct {
	Nested nestedTargets
}

type nestedTargets struct {
	Targets []discovery.Target
}
