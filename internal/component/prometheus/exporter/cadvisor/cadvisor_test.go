package cadvisor

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/integrations/cadvisor"
	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalRiver(t *testing.T) {
	riverCfg := `
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
`
	var args Arguments
	err := river.Unmarshal([]byte(riverCfg), &args)
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
	}
	require.Equal(t, expected, res)
}
