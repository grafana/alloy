package cadvisor

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
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

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
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
	}

	return cfg
}
