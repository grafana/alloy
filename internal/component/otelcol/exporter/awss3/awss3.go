// Package awss3 provides an otelcol.exporter.awss3 component
package awss3

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.awss3",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := awss3exporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.awss3 component.
type Arguments struct {
	Region                string                      `alloy:"region,attr"`
	S3Bucket              string                      `alloy:"s3_bucket,attr"`
	S3Prefix              string                      `alloy:"s3_prefix,attr"`
	S3Partition           string                      `alloy:"s3_partition,attr,optional"`
	RoleArn               string                      `alloy:"role_arn,attr,optional"`
	FilePrefix            string                      `alloy:"file_prefix,attr,optional"`
	Marshaler             awss3exporter.MarshalerType `alloy:"marshaler,attr,optional"`
	Encoding              string                      `alloy:"encoding,attr,optional"`
	EncodingFileExtension string                      `alloy:"encoding_file_ext,attr,optional"`
	Endpoint              string                      `alloy:"endpoint,attr"`
	S3ForcePathStyle      bool                        `alloy:"s3_force_path_style,attr,optional"`
	DisableSSL            bool                        `alloy:"disable_ssl,attr,optional"`
	Compression           configcompression.Type      `alloy:"compression,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional
	DebugMetrics otelcol.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Region:           "us-east-1",
		Marshaler:        "otlp_json",
		S3ForcePathStyle: false,
		DisableSSL:       false,
		Compression:      configcompression.TypeGzip,
	}
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]interface{})

	var result awss3exporter.Config
	err := mapstructure.Decode(input, &result)
	if err != nil {
		return nil, err
	}

	result.S3Uploader.Region = args.Region
	result.S3Uploader.S3Bucket = args.S3Bucket
	result.S3Uploader.S3Prefix = args.S3Prefix

	result.MarshalerName = args.Marshaler

	return &result, nil
}

func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}
