// Package file_stats provides an otelcol.receiver.file_stats component.
package file_stats

import (
	"fmt"
	"regexp"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.file_stats",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := filestatsreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.file_stats component.
type Arguments struct {
	Include string `alloy:"include,attr"`

	Controller     otelcol.ControllerArguments `alloy:",squash"`
	MetricsBuilder MetricsBuilderArguments     `alloy:",squash"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.Controller.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implemenets syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Include == "" {
		return fmt.Errorf("include must not be empty")
	}
	return nil
}

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
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

// MetricsBuilderArguments is a configuration for file_stats metrics builder.
type MetricsBuilderArguments struct {
	Metrics            MetricsArguments            `alloy:"metrics,block,optional"`
	ResourceAttributes ResourceAttributesArguments `alloy:"resource_attributes,block,optional"`
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *MetricsBuilderArguments) toMap() map[string]any {
	return map[string]any{
		"metrics":             args.Metrics.toMap(),
		"resource_attributes": args.ResourceAttributes.toMap(),
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *MetricsBuilderArguments) SetToDefault() {
	*args = MetricsBuilderArguments{}
	args.Metrics.SetToDefault()
	args.ResourceAttributes.SetToDefault()
}

// MetricsArguments provides config for file_stats metrics.
type MetricsArguments struct {
	FileAtime MetricArguments `alloy:"file.atime,block,optional"`
	FileCount MetricArguments `alloy:"file.count,block,optional"`
	FileCtime MetricArguments `alloy:"file.ctime,block,optional"`
	FileMtime MetricArguments `alloy:"file.mtime,block,optional"`
	FileSize  MetricArguments `alloy:"file.size,block,optional"`
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

// SetToDefault implements syntax.Defaulter.
func (args *MetricsArguments) SetToDefault() {
	args.FileAtime.Enabled = false
	args.FileCount.Enabled = false
	args.FileCtime.Enabled = false
	args.FileMtime.Enabled = true
	args.FileSize.Enabled = true
}

// MetricArguments provides common config for a particular metric.
type MetricArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *MetricArguments) toMap() map[string]any {
	return map[string]any{"enabled": args.Enabled}
}

// ResourceATtributesArguments provides config for file_stats resource
// attributes.
type ResourceAttributesArguments struct {
	FileName ResourceAttributeArguments `alloy:"file.name,block,optional"`
	FilePath ResourceAttributeArguments `alloy:"file.path,block,optional"`
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *ResourceAttributesArguments) toMap() map[string]any {
	return map[string]any{
		"file.name": args.FileName.toMap(),
		"file.path": args.FilePath.toMap(),
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *ResourceAttributesArguments) SetToDefault() {
	*args = ResourceAttributesArguments{}
	args.FileName.Enabled = true
	args.FilePath.Enabled = false
}

// ResourceAttributeArguments provides common config for a particular resource
// attribute.
type ResourceAttributeArguments struct {
	Enabled        bool              `alloy:"enabled,attr,optional"`
	MetricsInclude []FilterArguments `alloy:"metrics_include,block,optional"`
	MetricsExclude []FilterArguments `alloy:"metrics_exclude,block,optional"`
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

	return map[string]any{
		"enabled":         args.Enabled,
		"metrics_include": includes,
		"metrics_exclude": excludes,
	}
}

// FilterArguments configures the matching behavior of a FilterSet.
type FilterArguments struct {
	Strict string `alloy:"strict,attr,optional"`
	Regex  string `alloy:"regexp,attr,optional"`
}

// toMap encodes args to a map for use with mapstructure.Decode.
func (args *FilterArguments) toMap() map[string]any {
	return map[string]any{
		"strict": args.Strict,
		"regexp": args.Regex,
	}
}

// Validate implements syntax.Validator.
func (args *FilterArguments) Validate() error {
	if args.Strict == "" && args.Regex == "" {
		return fmt.Errorf("must specify either strict or regexp")
	}
	if args.Strict != "" && args.Regex != "" {
		return fmt.Errorf("strict and regexp are mutually exclusive")
	}

	if args.Regex != "" {
		_, err := regexp.Compile(args.Regex)
		if err != nil {
			return fmt.Errorf("parsing regexp: %w", err)
		}
	}

	return nil
}
