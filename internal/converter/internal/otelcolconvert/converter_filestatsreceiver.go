//go:build linux || darwin || windows

package otelcolconvert

// TODO(rfratto): Remove build directive above once FreeBSD is supported.

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/file_stats"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func init() {
	converters = append(converters, filestatsReceiverConverter{})
}

type filestatsReceiverConverter struct{}

func (filestatsReceiverConverter) Factory() component.Factory { return filestatsreceiver.NewFactory() }

func (filestatsReceiverConverter) InputComponentName() string { return "" }

func (filestatsReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toFilestatsReceiver(state, id, cfg.(*filestatsreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "file_stats"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toFilestatsReceiver(state *State, id componentstatus.InstanceID, cfg *filestatsreceiver.Config) *file_stats.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
	)

	return &file_stats.Arguments{
		Include: cfg.Include,

		Controller:     toScraperControllerArguments(cfg.ControllerConfig),
		MetricsBuilder: toFilestatsMetricsBuilderArguments(encodeMapstruct(cfg.MetricsBuilderConfig)),

		DebugMetrics: common.DefaultValue[file_stats.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},
	}
}

func toScraperControllerArguments(cfg scraperhelper.ControllerConfig) otelcol.ControllerArguments {
	return otelcol.ControllerArguments{
		CollectionInterval: cfg.CollectionInterval,
		InitialDelay:       cfg.InitialDelay,
		Timeout:            cfg.Timeout,
	}
}

func toFilestatsMetricsBuilderArguments(cfg map[string]any) file_stats.MetricsBuilderArguments {
	return file_stats.MetricsBuilderArguments{
		Metrics:            toFilestatsMetricsArguments(encodeMapstruct(cfg["metrics"])),
		ResourceAttributes: toFilestatsResourceAttributesArguments(encodeMapstruct(cfg["resource_attributes"])),
	}
}

func toFilestatsMetricsArguments(cfg map[string]any) file_stats.MetricsArguments {
	return file_stats.MetricsArguments{
		FileAtime: toFilestatsMetricArguments(encodeMapstruct(cfg["file.atime"])),
		FileCount: toFilestatsMetricArguments(encodeMapstruct(cfg["file.count"])),
		FileCtime: toFilestatsMetricArguments(encodeMapstruct(cfg["file.ctime"])),
		FileMtime: toFilestatsMetricArguments(encodeMapstruct(cfg["file.mtime"])),
		FileSize:  toFilestatsMetricArguments(encodeMapstruct(cfg["file.size"])),
	}
}

func toFilestatsMetricArguments(cfg map[string]any) file_stats.MetricArguments {
	return file_stats.MetricArguments{Enabled: cfg["enabled"].(bool)}
}

func toFilestatsResourceAttributesArguments(cfg map[string]any) file_stats.ResourceAttributesArguments {
	return file_stats.ResourceAttributesArguments{
		FileName: toFilestatsResourceAttributeArguments(encodeMapstruct(cfg["file.name"])),
		FilePath: toFilestatsResourceAttributeArguments(encodeMapstruct(cfg["file.path"])),
	}
}

func toFilestatsResourceAttributeArguments(cfg map[string]any) file_stats.ResourceAttributeArguments {
	var (
		includeSlice = encodeMapslice(cfg["metrics_include"])
		excludeSlice = encodeMapslice(cfg["metrics_exclude"])

		metricsInclude = make([]file_stats.FilterArguments, 0, len(includeSlice))
		metricsExclude = make([]file_stats.FilterArguments, 0, len(excludeSlice))
	)

	for _, include := range includeSlice {
		metricsInclude = append(metricsInclude, toFilestatsFilterArguments(include))
	}
	for _, exclude := range excludeSlice {
		metricsExclude = append(metricsExclude, toFilestatsFilterArguments(exclude))
	}

	return file_stats.ResourceAttributeArguments{
		Enabled:        cfg["enabled"].(bool),
		MetricsInclude: metricsInclude,
		MetricsExclude: metricsExclude,
	}
}

func toFilestatsFilterArguments(cfg map[string]any) file_stats.FilterArguments {
	return file_stats.FilterArguments{
		Strict: cfg["strict"].(string),
		Regex:  cfg["regexp"].(string),
	}
}
