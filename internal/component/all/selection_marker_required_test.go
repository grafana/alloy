//go:build !alloy_custom_components && alloy_component_local_file

package all

import (
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/stretchr/testify/require"
)

func TestComponentTagWithoutCustomMarkerKeepsFullRegistry(t *testing.T) {
	for _, name := range []string{"local.file", "prometheus.scrape", "pyroscope.write"} {
		_, exists := component.Get(name)
		require.True(t, exists, "expected full-build registration %q", name)
	}
}
