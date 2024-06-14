package build

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/cadvisor"
	cadvisor_integration "github.com/grafana/alloy/internal/static/integrations/cadvisor"
)

func (b *ConfigBuilder) appendCadvisorExporter(config *cadvisor_integration.Config, instanceKey *string) discovery.Exports {
	args := toCadvisorExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "cadvisor")
}

func toCadvisorExporter(config *cadvisor_integration.Config) *cadvisor.Arguments {
	return &cadvisor.Arguments{

		StoreContainerLabels:       config.StoreContainerLabels,
		AllowlistedContainerLabels: config.AllowlistedContainerLabels,
		EnvMetadataAllowlist:       config.EnvMetadataAllowlist,
		RawCgroupPrefixAllowlist:   config.RawCgroupPrefixAllowlist,
		PerfEventsConfig:           config.PerfEventsConfig,
		ResctrlInterval:            time.Duration(config.ResctrlInterval),
		DisabledMetrics:            config.DisabledMetrics,
		EnabledMetrics:             config.EnabledMetrics,
		StorageDuration:            config.StorageDuration,
	}
}
