//go:build windows

package windows_exporter

import (
	"testing"

	"github.com/prometheus-community/windows_exporter/pkg/collector"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestToWindowsExporterConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		cfg      Config
		expected collector.Config
	}{
		{
			name:     "empty config",
			cfg:      Config{},
			expected: collector.ConfigDefaults,
		},
		{
			name: "mscluster collectors enabled",
			cfg: Config{
				EnabledCollectors: "mscluster_cluster,mscluster_network,mscluster_node,mscluster_resource,mscluster_resourcegroup",
			},
			expected: func() collector.Config {
				cfg := collector.ConfigDefaults
				cfg.Mscluster.CollectorsEnabled = []string{"cluster", "network", "node", "resource", "resourcegroup"}
				return cfg
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			exporterConfig, err := tc.cfg.ToWindowsExporterConfig(enabledCollectors(tc.cfg.EnabledCollectors))
			require.NoError(t, err)

			assert.DeepEqual(t, tc.expected, exporterConfig)
		})
	}
}
