//go:build linux || darwin || windows

package file_stats

import (
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

var _ receiver.Arguments = Arguments{}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	var out filestatsreceiver.Config

	out.ControllerConfig = *args.Controller.Convert()
	out.Include = args.Include

	// We have to use mapstructure.Decode for args.MetricsBuilder because the
	// upstream type is in an internal package.
	err := mapstructure.Decode(
		args.MetricsBuilder.toMap(),
		&out.MetricsBuilderConfig,
	)

	return &out, err
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *MetricsBuilderArguments) toMap() map[string]any {
	return map[string]any{
		"metrics":             args.Metrics.toMap(),
		"resource_attributes": args.ResourceAttributes.toMap(),
	}
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *MetricsArguments) toMap() map[string]any {
	return map[string]any{
		"file.atime": args.FileAtime.toMap(),
		"file.count": args.FileCount.toMap(),
		"file.ctime": args.FileCtime.toMap(),
		"file.mtime": args.FileMtime.toMap(),
		"file.size":  args.FileSize.toMap(),
	}
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *MetricArguments) toMap() map[string]any {
	return map[string]any{"enabled": args.Enabled}
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *ResourceAttributesArguments) toMap() map[string]any {
	return map[string]any{
		"file.name": args.FileName.toMap(),
		"file.path": args.FilePath.toMap(),
	}
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *ResourceAttributeArguments) toMap() map[string]any {
	includes := make([]map[string]any, 0, len(args.MetricsInclude))
	excludes := make([]map[string]any, 0, len(args.MetricsExclude))

	for _, include := range args.MetricsInclude {
		includes = append(includes, include.toMap())
	}
	for _, exclude := range args.MetricsExclude {
		excludes = append(excludes, exclude.toMap())
	}

	// NOTE(rfratto): these **must** be nil if empty, otherwise the upstream
	// component will filter out all metrics.
	if len(includes) == 0 {
		includes = nil
	}
	if len(excludes) == 0 {
		excludes = nil
	}

	return map[string]any{
		"enabled":         args.Enabled,
		"metrics_include": includes,
		"metrics_exclude": excludes,
	}
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *FilterArguments) toMap() map[string]any {
	return map[string]any{
		"strict": args.Strict,
		"regexp": args.Regex,
	}
}
