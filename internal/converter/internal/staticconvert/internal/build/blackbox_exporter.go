package build

import (
	"time"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/blackbox"
	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	blackbox_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/blackbox_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendBlackboxExporter(config *blackbox_exporter.Config) discovery.Exports {
	args := toBlackboxExporter(config)
	return b.appendExporterBlock(args, config.Name(), nil, "blackbox")
}

func toBlackboxExporter(config *blackbox_exporter.Config) *blackbox.Arguments {
	return &blackbox.Arguments{
		ConfigFile: config.BlackboxConfigFile,
		Config: alloytypes.OptionalSecret{
			IsSecret: false,
			Value:    string(config.BlackboxConfig),
		},
		TargetsList:        toBlackboxTargets(config.BlackboxTargets),
		ProbeTimeoutOffset: time.Duration(config.ProbeTimeoutOffset),
	}
}

func (b *ConfigBuilder) appendBlackboxExporterV2(config *blackbox_exporter_v2.Config) discovery.Exports {
	args := toBlackboxExporterV2(config)
	return b.appendExporterBlock(args, config.Name(), config.Common.InstanceKey, "blackbox")
}

func toBlackboxExporterV2(config *blackbox_exporter_v2.Config) *blackbox.Arguments {
	return &blackbox.Arguments{
		ConfigFile: config.BlackboxConfigFile,
		Config: alloytypes.OptionalSecret{
			IsSecret: false,
			Value:    string(config.BlackboxConfig),
		},
		TargetsList:        toBlackboxTargets(config.BlackboxTargets),
		ProbeTimeoutOffset: time.Duration(config.ProbeTimeoutOffset),
	}
}

func toBlackboxTargets(blackboxTargets []blackbox_exporter.BlackboxTarget) []map[string]string {
	var targets blackbox.TargetsList
	for _, bt := range blackboxTargets {
		target := make(map[string]string)
		target["name"] = bt.Name
		target["address"] = bt.Target
		target["module"] = bt.Module
		targets = append(targets, target)
	}
	return targets
}
