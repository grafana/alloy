//go:build alloy_custom_components && alloy_component_loki_source_podlogs && !alloy_component_loki_source_kubernetes

package all

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildDoesNotRegisterTransitiveKubernetesSource(t *testing.T) {
	_, exists := component.Get("loki.source.podlogs")
	require.True(t, exists)

	_, exists = component.Get("loki.source.kubernetes")
	require.False(t, exists)
}
