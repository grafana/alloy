package build

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/blackbox"
	"github.com/grafana/agent/internal/static/integrations/blackbox_exporter"
	blackbox_exporter_v2 "github.com/grafana/agent/internal/static/integrations/v2/blackbox_exporter"
	"github.com/grafana/river/rivertypes"
)

func (b *ConfigBuilder) appendBlackboxExporter(config *blackbox_exporter.Config) discovery.Exports {
	args := toBlackboxExporter(config)
	return b.appendExporterBlock(args, config.Name(), nil, "blackbox")
}

func toBlackboxExporter(config *blackbox_exporter.Config) *blackbox.Arguments {
	return &blackbox.Arguments{
		ConfigFile: config.BlackboxConfigFile,
		Config: rivertypes.OptionalSecret{
			IsSecret: false,
			Value:    string(config.BlackboxConfig),
		},
		Targets:            toBlackboxTargets(config.BlackboxTargets),
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
		Config: rivertypes.OptionalSecret{
			IsSecret: false,
			Value:    string(config.BlackboxConfig),
		},
		Targets:            toBlackboxTargets(config.BlackboxTargets),
		ProbeTimeoutOffset: time.Duration(config.ProbeTimeoutOffset),
	}
}

func toBlackboxTargets(blackboxTargets []blackbox_exporter.BlackboxTarget) blackbox.TargetBlock {
	var targetBlock blackbox.TargetBlock

	for _, bt := range blackboxTargets {
		targetBlock = append(targetBlock, toBlackboxTarget(bt))
	}

	return targetBlock
}

func toBlackboxTarget(target blackbox_exporter.BlackboxTarget) blackbox.BlackboxTarget {
	return blackbox.BlackboxTarget{
		Name:   target.Name,
		Target: target.Target,
		Module: target.Module,
	}
}
