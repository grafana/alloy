// Package file provides an otelcol.exporter.file component.
package file

import (
	"errors"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.file",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := fileexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

// Arguments configures the otelcol.exporter.file component.
type Arguments struct {
	// Path of the file to write to. Path is relative to current directory.
	Path string `alloy:"path,attr"`

	// Append defines whether the exporter should append to the file.
	// Options:
	// - false[default]:  truncates the file
	// - true:  appends to the file.
	Append bool `alloy:"append,attr,optional"`

	// Rotation defines an option about rotation of telemetry files. Ignored
	// when GroupBy is enabled.
	Rotation *Rotation `alloy:"rotation,block,optional"`

	// FormatType define the data format of encoded telemetry data
	// Options:
	// - json[default]:  OTLP json bytes.
	// - proto:  OTLP binary protobuf bytes.
	Format string `alloy:"format,attr,optional"`

	// Encoding defines the encoding of the telemetry data.
	// If specified, it overrides `Format` and applies an encoding extension.
	Encoding string `alloy:"encoding,attr,optional"`

	// Compression Codec used to export telemetry data
	// Supported compression algorithms:`zstd`
	Compression string `alloy:"compression,attr,optional"`

	// FlushInterval is the duration between flushes.
	// See time.ParseDuration for valid values.
	FlushInterval time.Duration `alloy:"flush_interval,attr,optional"`

	// GroupBy enables writing to separate files based on a resource attribute.
	GroupBy *GroupBy `alloy:"group_by,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// Rotation defines an option to rolling log files
type Rotation struct {
	// MaxMegabytes is the maximum size in megabytes of the file before it gets
	// rotated. It defaults to 100 megabytes.
	MaxMegabytes int `alloy:"max_megabytes,attr,optional"`

	// MaxDays is the maximum number of days to retain old log files based on the
	// timestamp encoded in their filename.  Note that a day is defined as 24
	// hours and may not exactly correspond to calendar days due to daylight
	// savings, leap seconds, etc. The default is not to remove old log files
	// based on age.
	MaxDays int `alloy:"max_days,attr,optional"`

	// MaxBackups is the maximum number of old log files to retain. The default
	// is to 100 files.
	MaxBackups int `alloy:"max_backups,attr,optional"`

	// LocalTime determines if the time used for formatting the timestamps in
	// backup files is the computer's local time.  The default is to use UTC
	// time.
	LocalTime bool `alloy:"localtime,attr,optional"`
}

type GroupBy struct {
	// Enables group_by. When group_by is enabled, rotation setting is ignored. Default is false.
	Enabled bool `alloy:"enabled,attr,optional"`

	// ResourceAttribute specifies the name of the resource attribute that
	// contains the path segment of the file to write to. The final path will be
	// the Path config value, with the * replaced with the value of this resource
	// attribute. Default is "fileexporter.path_segment".
	ResourceAttribute string `alloy:"resource_attribute,attr,optional"`

	// MaxOpenFiles specifies the maximum number of open file descriptors for the output files.
	// The default is 100.
	MaxOpenFiles int `alloy:"max_open_files,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Format:        "json",
		FlushInterval: time.Second,
	}
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	cfg := &fileexporter.Config{
		Path:          args.Path,
		Append:        args.Append,
		FormatType:    args.Format,
		Compression:   args.Compression,
		FlushInterval: args.FlushInterval,
	}

	if args.Encoding != "" {
		// For now, we'll skip the encoding feature as it requires more complex component ID parsing
		// TODO: Implement proper encoding support
		return nil, errors.New("encoding parameter is not yet supported")
	}

	if args.Rotation != nil {
		cfg.Rotation = &fileexporter.Rotation{
			MaxMegabytes: args.Rotation.MaxMegabytes,
			MaxDays:      args.Rotation.MaxDays,
			MaxBackups:   args.Rotation.MaxBackups,
			LocalTime:    args.Rotation.LocalTime,
		}
	}

	if args.GroupBy != nil {
		cfg.GroupBy = &fileexporter.GroupBy{
			Enabled:           args.GroupBy.Enabled,
			ResourceAttribute: args.GroupBy.ResourceAttribute,
			MaxOpenFiles:      args.GroupBy.MaxOpenFiles,
		}
	}

	return cfg, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Path == "" {
		return errors.New("path must be non-empty")
	}

	if args.Append && args.Compression != "" {
		return errors.New("append and compression enabled at the same time is not supported")
	}

	if args.Append && args.Rotation != nil {
		return errors.New("append and rotation enabled at the same time is not supported")
	}

	if args.Format != "json" && args.Format != "proto" {
		return errors.New("format type must be json or proto")
	}

	if args.Compression != "" && args.Compression != "zstd" {
		return errors.New("compression must be zstd if specified")
	}

	if args.FlushInterval <= 0 {
		return errors.New("flush_interval must be greater than zero")
	}

	if args.GroupBy != nil && args.GroupBy.Enabled {
		// Apply implicit defaults if user enabled group_by but omitted optional fields.
		if args.GroupBy.ResourceAttribute == "" {
			args.GroupBy.ResourceAttribute = "fileexporter.path_segment"
		}
		if args.GroupBy.MaxOpenFiles == 0 {
			args.GroupBy.MaxOpenFiles = 100
		}
		pathParts := strings.Split(args.Path, "*")
		if len(pathParts) != 2 {
			return errors.New("path must contain exactly one * when group_by is enabled")
		}

		if pathParts[0] == "" {
			return errors.New("path must not start with * when group_by is enabled")
		}

		if args.GroupBy.ResourceAttribute == "" { // Should not happen after defaulting above, but keep for safety.
			return errors.New("resource_attribute must not be empty when group_by is enabled")
		}
	}

	return nil
}