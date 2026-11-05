package node_exporter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/node_exporter/collector"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/util"
)

func TestNodeExporter_QdiscDeviceFilter(t *testing.T) {
	fixtures := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fixtures, "results.json"), []byte(`[
		{"IfaceName":"eth0","Kind":"fq_codel","Bytes":42},
		{"IfaceName":"veth123","Kind":"fq_codel","Bytes":24}
	]`), 0600))

	for _, tc := range []struct {
		name    string
		config  string
		devices []string
	}{
		{name: "defaults", devices: []string{"eth0", "veth123"}},
		{name: "include", config: `qdisc_device_include: "^eth.*"`, devices: []string{"eth0"}},
		{name: "exclude", config: `qdisc_device_exclude: "^veth.*"`, devices: []string{"eth0"}},
		{name: "include no matches", config: `qdisc_device_include: "^bond.*"`},
		{name: "exclude all", config: `qdisc_device_exclude: ".*"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			require.NoError(t, yaml.Unmarshal([]byte(tc.config), &cfg))
			nodeConfig := cfg.mapConfigToNodeConfig()
			qdisc, err := collector.NewQdiscStatCollector(nodeConfig, util.TestAlloyLogger(t).Slog())
			require.NoError(t, err)

			// Qdisc initialization must not modify the shared empty settings.
			require.Empty(t, *nodeConfig.Qdisc.Fixtures)
			require.Empty(t, *nodeConfig.Qdisc.OldDeviceInclude)
			require.Empty(t, *nodeConfig.NetDev.OldDeviceExclude)
			nodeConfig.Qdisc.Fixtures = &fixtures

			metrics := make(chan prometheus.Metric)
			collectionErr := make(chan error, 1)
			go func() {
				collectionErr <- qdisc.Update(metrics)
				close(metrics)
			}()

			devices := map[string]bool{}
			for metric := range metrics {
				var value dto.Metric
				require.NoError(t, metric.Write(&value))
				for _, label := range value.Label {
					if label.GetName() == "device" {
						devices[label.GetValue()] = true
					}
				}
			}
			require.NoError(t, <-collectionErr)
			var got []string
			for device := range devices {
				got = append(got, device)
			}
			require.ElementsMatch(t, tc.devices, got)
		})
	}
}

func TestNodeExporter_QdiscMutuallyExclusiveDeviceFilters(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(`
set_collectors: [qdisc]
qdisc_device_include: "^eth.*"
qdisc_device_exclude: "^veth.*"
`), &cfg))
	_, err := New(util.TestAlloyLogger(t).Slog(), &cfg)
	require.ErrorContains(t, err, "collector.qdisc.device-include and collector.qdisc.device-exclude")
}
