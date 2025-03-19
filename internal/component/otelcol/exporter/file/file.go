// Package file providers an otelcol.exporter.file component.
package file

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.file",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := fileexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), GetSigalType)
		},
	})
}

func GetSigalType(opts component.Options, args component.Arguments) exporter.TypeSignal {
	return exporter.TypeAll
}

type Arguments struct {
	Path     string   `alloy:"path,attr"`
	Rotation Rotation `alloy:"rotation,attr,optional"`
	Format   string   `alloy:"format,attr,optional"`
	// We are not exposing the encoding for now; the input is coming from Alloy anyway.
	// Encoding      string        `alloy:"encoding,attr,optional"`
	Append        bool          `alloy:"append,attr,optional"`
	Compression   string        `alloy:"compression,attr,optional"`
	FlushInterval time.Duration `alloy:"flush_interval,attr,optional"`
	GroupBy       GroupBy       `alloy:"group_by,attr,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type Rotation struct {
	MaxMegabytes int  `alloy:"max_megabytes,attr,optional"`
	MaxDays      int  `alloy:"max_days,attr,optional"`
	MaxBackups   int  `alloy:"max_backups,attr,optional"`
	LocalTime    bool `alloy:"localtime,attr,optional"`
}

func (rotation Rotation) Convert() *fileexporter.Rotation {
	return &fileexporter.Rotation{
		MaxMegabytes: rotation.MaxMegabytes,
		MaxDays:      rotation.MaxDays,
		MaxBackups:   rotation.MaxBackups,
		LocalTime:    rotation.LocalTime,
	}
}

type GroupBy struct {
	Enabled           bool   `alloy:"enabled,attr"`
	ResourceAttribute string `alloy:"resource_attribute,attr"`
	MaxOpenFiles      int    `alloy:"max_open_files,attr,optional"`
}

func (groupBy GroupBy) Convert() *fileexporter.GroupBy {
	return &fileexporter.GroupBy{
		Enabled:           groupBy.Enabled,
		ResourceAttribute: groupBy.ResourceAttribute,
		MaxOpenFiles:      groupBy.MaxOpenFiles,
	}
}

var (
	_ syntax.Validator   = (*Arguments)(nil)
	_ syntax.Defaulter   = (*Arguments)(nil)
	_ exporter.Arguments = (*Arguments)(nil)
)

var DefaultArguments = Arguments{
	Rotation: Rotation{
		MaxMegabytes: 100,
		MaxBackups:   100,
		LocalTime:    false,
	},
	Format:        "json",
	Append:        false,
	FlushInterval: time.Second,
	GroupBy: GroupBy{
		Enabled:           false,
		ResourceAttribute: "fileexporter.path_segment",
		MaxOpenFiles:      100,
	},
}

func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

func (args *Arguments) Validate() error {
	// TODO: validate path is valid

	if args.Format != "" && args.Format != "json" && args.Format != "proto" {
		return fmt.Errorf("Format must be 'json' or 'proto'")
	}

	if args.Compression != "" && args.Compression != "zstd" {
		return fmt.Errorf("Compression must be 'zstd'")
	}

	return nil
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &fileexporter.Config{
		Path:          args.Path, // TODO: validate valid filepath
		Append:        args.Append,
		Rotation:      args.Rotation.Convert(),
		FormatType:    args.Format,
		Compression:   args.Compression,
		FlushInterval: args.FlushInterval,
		GroupBy:       args.GroupBy.Convert(),
	}, nil
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
