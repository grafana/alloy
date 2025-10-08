package cadvisor

import (
	"time"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

const name = "cadvisor"

// DefaultConfig holds the default settings for the cadvisor integration
var DefaultConfig = Config{
	// Common cadvisor config defaults
	StoreContainerLabels: true,
	ResctrlInterval:      0,

	StorageDuration: 2 * time.Minute,

	// Containerd config defaults
	Containerd:          "/run/containerd/containerd.sock",
	ContainerdNamespace: "k8s.io",

	// Docker config defaults
	Docker:        "unix:///var/run/docker.sock",
	DockerTLS:     false,
	DockerTLSCert: "cert.pem",
	DockerTLSKey:  "key.pem",
	DockerTLSCA:   "ca.pem",

	// Raw config defaults
	DockerOnly: false,

	DisableRootCgroupStats: false,
}

// Config controls cadvisor
type Config struct {
	// Common cadvisor config options
	// StoreContainerLabels converts container labels and environment variables into labels on prometheus metrics for each container. If false, then only metrics exported are container name, first alias, and image name.
	StoreContainerLabels bool `yaml:"store_container_labels,omitempty"`

	// AllowlistedContainerLabels list of container labels to be converted to labels on prometheus metrics for each container. store_container_labels must be set to false for this to take effect.
	AllowlistedContainerLabels []string `yaml:"allowlisted_container_labels,omitempty"`

	// EnvMetadataAllowlist list of environment variable keys matched with specified prefix that needs to be collected for containers, only support containerd and docker runtime for now.
	EnvMetadataAllowlist []string `yaml:"env_metadata_allowlist,omitempty"`

	// RawCgroupPrefixAllowlist list of cgroup path prefix that needs to be collected even when -docker_only is specified.
	RawCgroupPrefixAllowlist []string `yaml:"raw_cgroup_prefix_allowlist,omitempty"`

	// PerfEventsConfig path to a JSON file containing configuration of perf events to measure. Empty value disabled perf events measuring.
	PerfEventsConfig string `yaml:"perf_events_config,omitempty"`

	// ResctrlInterval resctrl mon groups updating interval. Zero value disables updating mon groups.
	ResctrlInterval int64 `yaml:"resctrl_interval,omitempty"`

	// DisableMetrics list of `metrics` to be disabled.
	DisabledMetrics []string `yaml:"disabled_metrics,omitempty"`

	// EnableMetrics list of `metrics` to be enabled. If set, overrides 'disable_metrics'.
	EnabledMetrics []string `yaml:"enabled_metrics,omitempty"`

	// StorageDuration length of time to keep data stored in memory (Default: 2m)
	StorageDuration time.Duration `yaml:"storage_duration,omitempty"`

	// Containerd config options
	// Containerd containerd endpoint
	Containerd string `yaml:"containerd,omitempty"`

	// ContainerdNamespace containerd namespace
	ContainerdNamespace string `yaml:"containerd_namespace,omitempty"`

	// Docker config options
	// Docker docker endpoint
	Docker string `yaml:"docker,omitempty"`

	// DockerTLS use TLS to connect to docker
	DockerTLS bool `yaml:"docker_tls,omitempty"`

	// DockerTLSCert path to client certificate
	DockerTLSCert string `yaml:"docker_tls_cert,omitempty"`

	// DockerTLSKey path to private key
	DockerTLSKey string `yaml:"docker_tls_key,omitempty"`

	// DockerTLSCA path to trusted CA
	DockerTLSCA string `yaml:"docker_tls_ca,omitempty"`

	// Raw config options
	// DockerOnly only report docker containers in addition to root stats
	DockerOnly bool `yaml:"docker_only,omitempty"`

	// DisableRootCgroupStats informs the exporter not collecting stats from the root cgroup.
	DisableRootCgroupStats bool `yaml:"disable_root_cgroup_stats,omitempty"`

	// Hold on to the logger passed to config.NewIntegration, to be passed to klog, as yet another unsafe global that needs to be set.
	logger log.Logger //nolint:unused,structcheck // logger is only used on linux
}

// UnmarshalYAML implements yaml.Unmarshaler for Config
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = DefaultConfig

	type plain Config
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}

	// In the cadvisor cmd, these are passed as CSVs, and turned into slices using strings.split. As a result the
	// default values are always a slice with 1 or more elements.
	// See: https://github.com/google/cadvisor/blob/v0.43.0/cmd/cadvisor.go#L136
	if len(c.AllowlistedContainerLabels) == 0 {
		c.AllowlistedContainerLabels = []string{""}
	}
	if len(c.RawCgroupPrefixAllowlist) == 0 {
		c.RawCgroupPrefixAllowlist = []string{""}
	}
	if len(c.EnvMetadataAllowlist) == 0 {
		c.EnvMetadataAllowlist = []string{""}
	}
	return nil
}

// Name returns the name of the integration that this config represents.
func (c *Config) Name() string {
	return name
}

func (c *Config) InstanceKey(defaultKey string) (string, error) {
	return defaultKey, nil
}

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeSingleton, metricsutils.Shim)
}
