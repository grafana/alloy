package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/process"
	"github.com/grafana/alloy/internal/static/integrations/process_exporter"
)

func (b *ConfigBuilder) appendProcessExporter(config *process_exporter.Config, instanceKey *string) discovery.Exports {
	args := toProcessExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "process")
}

func toProcessExporter(config *process_exporter.Config) *process.Arguments {
	matcherGroups := make([]process.MatcherGroup, 0)
	for _, matcherGroup := range config.ProcessExporter {
		matcherGroups = append(matcherGroups, process.MatcherGroup{
			Name:         matcherGroup.Name,
			CommRules:    matcherGroup.CommRules,
			ExeRules:     matcherGroup.ExeRules,
			CmdlineRules: matcherGroup.CmdlineRules,
		})
	}

	return &process.Arguments{
		ProcessExporter:   matcherGroups,
		ProcFSPath:        config.ProcFSPath,
		Children:          config.Children,
		Threads:           config.Threads,
		SMaps:             config.SMaps,
		Recheck:           config.Recheck,
		RemoveEmptyGroups: config.RemoveEmptyGroups,
	}
}
