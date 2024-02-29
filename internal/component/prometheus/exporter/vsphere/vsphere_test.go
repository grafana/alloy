package vsphere

import (
	"testing"

	"github.com/grafana/agent/internal/static/integrations/vmware_exporter"
	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
		request_chunk_size = 256
		collect_concurrency = 8
		vsphere_url = "https://localhost:443/sdk"
		vsphere_user = "user"
		vsphere_password = "pass"
		discovery_interval = 0
		enable_exporter_metrics = true
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)

	require.NoError(t, err)
	expected := Arguments{
		ChunkSize:               256,
		CollectConcurrency:      8,
		VSphereURL:              "https://localhost:443/sdk",
		VSphereUser:             "user",
		VSpherePass:             "pass",
		ObjectDiscoveryInterval: 0,
		EnableExporterMetrics:   true,
	}
	require.Equal(t, expected, args)
}

func TestRiverConvert(t *testing.T) {
	orig := Arguments{
		ChunkSize:               256,
		CollectConcurrency:      8,
		VSphereURL:              "https://localhost:443/sdk",
		VSphereUser:             "user",
		VSpherePass:             "pass",
		ObjectDiscoveryInterval: 0,
		EnableExporterMetrics:   true,
	}
	converted := orig.Convert()
	expected := vmware_exporter.Config{
		ChunkSize:               256,
		CollectConcurrency:      8,
		VSphereURL:              "https://localhost:443/sdk",
		VSphereUser:             "user",
		VSpherePass:             "pass",
		ObjectDiscoveryInterval: 0,
		EnableExporterMetrics:   true,
	}

	require.Equal(t, expected, *converted)
}
