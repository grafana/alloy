//go:build alloy_custom_components && alloy_component_database_observability_mysql && !alloy_component_prometheus_exporter_mysql

package all

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildDoesNotRegisterTransitiveMySQLExporter(t *testing.T) {
	_, exists := component.Get("database_observability.mysql")
	require.True(t, exists)

	_, exists = component.Get("prometheus.exporter.mysql")
	require.False(t, exists)
}
