package cadvisor

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/static/integrations/cadvisor"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalAlloy(t *testing.T) {
	alloyCfg := `
store_container_labels = true
allowlisted_container_labels = ["label1", "label2"]
env_metadata_allowlist = ["env1", "env2"]
raw_cgroup_prefix_allowlist = ["prefix1", "prefix2"]
perf_events_config = "perf_events_config"
resctrl_interval = "1s"
disabled_metrics = ["metric1", "metric2"]
enabled_metrics = ["metric3", "metric4"]
storage_duration = "2s"
containerd_host = "containerd_host"
containerd_namespace = "containerd_namespace"
docker_host = "docker_host"
use_docker_tls = true
docker_tls_cert = "docker_tls_cert"
docker_tls_key = "docker_tls_key"
docker_tls_ca = "docker_tls_ca"
housekeeping_interval = "15s"
max_housekeeping_interval = "2m"
allow_dynamic_housekeeping = false
`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyCfg), &args)
	require.NoError(t, err)
	expected := Arguments{
		StoreContainerLabels:       true,
		AllowlistedContainerLabels: []string{"label1", "label2"},
		EnvMetadataAllowlist:       []string{"env1", "env2"},
		RawCgroupPrefixAllowlist:   []string{"prefix1", "prefix2"},
		PerfEventsConfig:           "perf_events_config",
		ResctrlInterval:            1 * time.Second,
		DisabledMetrics:            []string{"metric1", "metric2"},
		EnabledMetrics:             []string{"metric3", "metric4"},
		StorageDuration:            2 * time.Second,
		ContainerdHost:             "containerd_host",
		ContainerdNamespace:        "containerd_namespace",
		DockerHost:                 "docker_host",
		UseDockerTLS:               true,
		DockerTLSCert:              "docker_tls_cert",
		DockerTLSKey:               "docker_tls_key",
		DockerTLSCA:                "docker_tls_ca",
		HousekeepingInterval:       15 * time.Second,
		MaxHousekeepingInterval:    2 * time.Minute,
		AllowDynamicHousekeeping:   false,
	}
	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	args := Arguments{
		StoreContainerLabels:       true,
		AllowlistedContainerLabels: []string{"label1", "label2"},
		EnvMetadataAllowlist:       []string{"env1", "env2"},
		RawCgroupPrefixAllowlist:   []string{"prefix1", "prefix2"},
		PerfEventsConfig:           "perf_events_config",
		ResctrlInterval:            1 * time.Second,
		DisabledMetrics:            []string{"metric1", "metric2"},
		EnabledMetrics:             []string{"metric3", "metric4"},
		StorageDuration:            2 * time.Second,
		ContainerdHost:             "containerd_host",
		ContainerdNamespace:        "containerd_namespace",
		DockerHost:                 "docker_host",
		UseDockerTLS:               true,
		DockerTLSCert:              "docker_tls_cert",
		DockerTLSKey:               "docker_tls_key",
		DockerTLSCA:                "docker_tls_ca",
		HousekeepingInterval:       15 * time.Second,
		MaxHousekeepingInterval:    2 * time.Minute,
		AllowDynamicHousekeeping:   false,
	}

	res := args.Convert()
	expected := &cadvisor.Config{
		StoreContainerLabels:       true,
		AllowlistedContainerLabels: []string{"label1", "label2"},
		EnvMetadataAllowlist:       []string{"env1", "env2"},
		RawCgroupPrefixAllowlist:   []string{"prefix1", "prefix2"},
		PerfEventsConfig:           "perf_events_config",
		ResctrlInterval:            int64(1 * time.Second),
		DisabledMetrics:            []string{"metric1", "metric2"},
		EnabledMetrics:             []string{"metric3", "metric4"},
		StorageDuration:            2 * time.Second,
		Containerd:                 "containerd_host",
		ContainerdNamespace:        "containerd_namespace",
		Docker:                     "docker_host",
		DockerTLS:                  true,
		DockerTLSCert:              "docker_tls_cert",
		DockerTLSKey:               "docker_tls_key",
		DockerTLSCA:                "docker_tls_ca",
		HousekeepingInterval:       15 * time.Second,
		MaxHousekeepingInterval:    2 * time.Minute,
		AllowDynamicHousekeeping:   false,
	}
	require.Equal(t, expected, res)
}

func TestHousekeepingDefaults(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(``), &args))
	require.Equal(t, 1*time.Second, args.HousekeepingInterval)
	require.Equal(t, 60*time.Second, args.MaxHousekeepingInterval)
	require.True(t, args.AllowDynamicHousekeeping)
}

func TestValidateHousekeeping(t *testing.T) {
	tests := []struct {
		name    string
		cfg     string
		wantErr string
	}{
		{name: "valid", cfg: `housekeeping_interval = "15s"`},
		{name: "zero interval", cfg: `housekeeping_interval = "0s"`, wantErr: "housekeeping_interval must be greater than 0"},
		{name: "max below base", cfg: "housekeeping_interval = \"2m\"\nmax_housekeeping_interval = \"1m\"", wantErr: "max_housekeeping_interval (1m0s) must be greater than or equal to housekeeping_interval (2m0s)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
