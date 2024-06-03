package file_stats

import (
	"fmt"
	"regexp"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/syntax"
)

// Arguments configures the otelcol.receiver.file_stats component.
type Arguments struct {
	Include string `alloy:"include,attr"`

	Controller     otelcol.ControllerArguments `alloy:",squash"`
	MetricsBuilder MetricsBuilderArguments     `alloy:",squash"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var (
	_ syntax.Defaulter = (*Arguments)(nil)
	_ syntax.Validator = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.Controller.SetToDefault()
	args.MetricsBuilder.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implemenets syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Include == "" {
		return fmt.Errorf("include must not be empty")
	}
	return nil
}

// MetricsBuilderArguments is a configuration for file_stats metrics builder.
type MetricsBuilderArguments struct {
	Metrics            MetricsArguments            `alloy:"metrics,block,optional"`
	ResourceAttributes ResourceAttributesArguments `alloy:"resource_attributes,block,optional"`
}

var (
	_ syntax.Defaulter = (*MetricsBuilderArguments)(nil)
)

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

var (
	_ syntax.Defaulter = (*MetricsArguments)(nil)
)

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

// ResourceATtributesArguments provides config for file_stats resource
// attributes.
type ResourceAttributesArguments struct {
	FileName ResourceAttributeArguments `alloy:"file.name,block,optional"`
	FilePath ResourceAttributeArguments `alloy:"file.path,block,optional"`
}

var (
	_ syntax.Defaulter = (*ResourceAttributesArguments)(nil)
)

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

// FilterArguments configures the matching behavior of a FilterSet.
type FilterArguments struct {
	Strict string `alloy:"strict,attr,optional"`
	Regex  string `alloy:"regexp,attr,optional"`
}

var _ syntax.Validator = (*FilterArguments)(nil)

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
