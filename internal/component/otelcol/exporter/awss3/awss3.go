// Package awss3 provides an otelcol.exporter.awss3 component
package awss3

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.awss3",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := awss3exporter.NewFactory()

			typeSignalFunc := func(opts component.Options, args component.Arguments) exporter.TypeSignal {
				switch args.(Arguments).MarshalerName.Type {
				case "sumo_ic":
					return exporter.TypeLogs
				default:
					return exporter.TypeAll
				}
			}

			return exporter.New(opts, fact, args.(Arguments), typeSignalFunc)
		},
	})
}

// Arguments configures the otelcol.exporter.awss3 component.
type Arguments struct {
	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`

	S3Uploader    S3Uploader    `alloy:"s3_uploader,block"`
	MarshalerName MarshalerType `alloy:"marshaler,block,optional"`

	// DebugMetrics configures component internal metrics. Optional
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	args.MarshalerName.SetToDefault()
	args.S3Uploader.SetToDefault()
	args.DebugMetrics.SetToDefault()
	args.Queue.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	var result awss3exporter.Config

	result.S3Uploader = args.S3Uploader.Convert()
	result.MarshalerName = args.MarshalerName.Convert()
	result.QueueSettings = *args.Queue.Convert()

	return &result, nil
}

func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// S3 Uploader Arguments Block
type S3Uploader struct {
	Region           string                 `alloy:"region,attr,optional"`
	S3Bucket         string                 `alloy:"s3_bucket,attr"`
	S3Prefix         string                 `alloy:"s3_prefix,attr"`
	S3Partition      string                 `alloy:"s3_partition,attr,optional"`
	RoleArn          string                 `alloy:"role_arn,attr,optional"`
	FilePrefix       string                 `alloy:"file_prefix,attr,optional"`
	Endpoint         string                 `alloy:"endpoint,attr,optional"`
	S3ForcePathStyle bool                   `alloy:"s3_force_path_style,attr,optional"`
	DisableSSL       bool                   `alloy:"disable_ssl,attr,optional"`
	Compression      configcompression.Type `alloy:"compression,attr,optional"`
}

func (args *S3Uploader) SetToDefault() {
	*args = S3Uploader{
		Region:           "us-east-1",
		S3ForcePathStyle: false,
		DisableSSL:       false,
	}
}

func (args *S3Uploader) Convert() awss3exporter.S3UploaderConfig {
	return awss3exporter.S3UploaderConfig{
		Region:           args.Region,
		S3Bucket:         args.S3Bucket,
		S3Prefix:         args.S3Prefix,
		S3Partition:      args.S3Partition,
		FilePrefix:       args.FilePrefix,
		Endpoint:         args.Endpoint,
		RoleArn:          args.RoleArn,
		S3ForcePathStyle: args.S3ForcePathStyle,
		DisableSSL:       args.DisableSSL,
	}
}

// MarshalerType Argument Block
type MarshalerType struct {
	Type string `alloy:"type,attr,optional"`
}

func (args *MarshalerType) SetToDefault() {
	*args = MarshalerType{
		Type: "otlp_json",
	}
}

func (args MarshalerType) Convert() awss3exporter.MarshalerType {
	return awss3exporter.MarshalerType(args.Type)
}
