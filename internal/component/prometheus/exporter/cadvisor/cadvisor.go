package cadvisor

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/common"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/cadvisor"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.cadvisor",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "cadvisor"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := common.HostNameInstanceKey() // if cannot resolve instance key, use the host name for cadvisor
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// Arguments configures the prometheus.exporter.cadvisor component.
type Arguments struct {
	StoreContainerLabels       bool          `alloy:"store_container_labels,attr,optional"`
	AllowlistedContainerLabels []string      `alloy:"allowlisted_container_labels,attr,optional"`
	EnvMetadataAllowlist       []string      `alloy:"env_metadata_allowlist,attr,optional"`
	RawCgroupPrefixAllowlist   []string      `alloy:"raw_cgroup_prefix_allowlist,attr,optional"`
	PerfEventsConfig           string        `alloy:"perf_events_config,attr,optional"`
	ResctrlInterval            time.Duration `alloy:"resctrl_interval,attr,optional"`
	DisabledMetrics            []string      `alloy:"disabled_metrics,attr,optional"`
	EnabledMetrics             []string      `alloy:"enabled_metrics,attr,optional"`
	StorageDuration            time.Duration `alloy:"storage_duration,attr,optional"`
	ContainerdHost             string        `alloy:"containerd_host,attr,optional"`
	ContainerdNamespace        string        `alloy:"containerd_namespace,attr,optional"`
	DockerHost                 string        `alloy:"docker_host,attr,optional"`
	UseDockerTLS               bool          `alloy:"use_docker_tls,attr,optional"`
	DockerTLSCert              string        `alloy:"docker_tls_cert,attr,optional"`
	DockerTLSKey               string        `alloy:"docker_tls_key,attr,optional"`
	DockerTLSCA                string        `alloy:"docker_tls_ca,attr,optional"`
	DockerOnly                 bool          `alloy:"docker_only,attr,optional"`
	DisableRootCgroupStats     bool          `alloy:"disable_root_cgroup_stats,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		StoreContainerLabels:       true,
		AllowlistedContainerLabels: []string{""},
		EnvMetadataAllowlist:       []string{""},
		RawCgroupPrefixAllowlist:   []string{""},
		ResctrlInterval:            0,
		StorageDuration:            2 * time.Minute,

		ContainerdHost:      "/run/containerd/containerd.sock",
		ContainerdNamespace: "k8s.io",

		// TODO(@tpaschalis) Do we need the default cert/key/ca since tls is disabled by default?
		DockerHost:    "unix:///var/run/docker.sock",
		UseDockerTLS:  false,
		DockerTLSCert: "cert.pem",
		DockerTLSKey:  "key.pem",
		DockerTLSCA:   "ca.pem",

		DockerOnly:             false,
		DisableRootCgroupStats: false,
	}
}

// Convert returns the upstream-compatible configuration struct.
func (a *Arguments) Convert() *cadvisor.Config {
	if len(a.AllowlistedContainerLabels) == 0 {
		a.AllowlistedContainerLabels = []string{""}
	}
	if len(a.RawCgroupPrefixAllowlist) == 0 {
		a.RawCgroupPrefixAllowlist = []string{""}
	}
	if len(a.EnvMetadataAllowlist) == 0 {
		a.EnvMetadataAllowlist = []string{""}
	}

	cfg := &cadvisor.Config{
		StoreContainerLabels:       a.StoreContainerLabels,
		AllowlistedContainerLabels: a.AllowlistedContainerLabels,
		EnvMetadataAllowlist:       a.EnvMetadataAllowlist,
		RawCgroupPrefixAllowlist:   a.RawCgroupPrefixAllowlist,
		PerfEventsConfig:           a.PerfEventsConfig,
		ResctrlInterval:            int64(a.ResctrlInterval), // TODO(@tpaschalis) This is so that the cadvisor package can re-cast back to time.Duration. Can we make it use time.Duration directly instead?
		DisabledMetrics:            a.DisabledMetrics,
		EnabledMetrics:             a.EnabledMetrics,
		StorageDuration:            a.StorageDuration,
		Containerd:                 a.ContainerdHost,
		ContainerdNamespace:        a.ContainerdNamespace,
		Docker:                     a.DockerHost,
		DockerTLS:                  a.UseDockerTLS,
		DockerTLSCert:              a.DockerTLSCert,
		DockerTLSKey:               a.DockerTLSKey,
		DockerTLSCA:                a.DockerTLSCA,
		DockerOnly:                 a.DockerOnly,
		DisableRootCgroupStats:     a.DisableRootCgroupStats,
	}

	return cfg
}
