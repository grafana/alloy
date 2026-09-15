//go:build alloy_custom_components && alloy_component_pyroscope_scrape && !alloy_component_prometheus_scrape

package all

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestCustomBuildDoesNotRegisterTransitivePrometheusScrape(t *testing.T) {
	_, exists := component.Get("pyroscope.scrape")
	require.True(t, exists)

	_, exists = component.Get("prometheus.scrape")
	require.False(t, exists)
}
