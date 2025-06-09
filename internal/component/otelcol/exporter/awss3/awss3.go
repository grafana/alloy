// Package awss3 provides an otelcol.exporter.awss3 component
package awss3

import (
	"time"

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
	Timeout       time.Duration `alloy:"timeout,attr,optional"`

	ResourceAttrsToS3 ResourceAttrsToS3 `alloy:"resource_attrs_to_s3,block,optional"`

	// DebugMetrics configures component internal metrics. Optional
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	args.MarshalerName.SetToDefault()
	args.S3Uploader.SetToDefault()
	args.DebugMetrics.SetToDefault()
	args.Queue.SetToDefault()
	args.Timeout = otelcol.DefaultTimeout
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}
	awss3Cfg := otelCfg.(*awss3exporter.Config)
	return awss3Cfg.Validate()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	var result awss3exporter.Config

	result.S3Uploader = args.S3Uploader.Convert()
	result.MarshalerName = args.MarshalerName.Convert()
	result.ResourceAttrsToS3 = args.ResourceAttrsToS3.Convert()
	result.TimeoutSettings.Timeout = args.Timeout

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	result.QueueSettings = *q

	return &result, nil
}

func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.Queue.Extensions()
}

func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// ResourceAttrsToS3 defines the mapping of S3 uploading configuration values to resource attribute values.
type ResourceAttrsToS3 struct {
	// S3Prefix indicates the mapping of the key (directory) prefix used for writing into the bucket to a specific resource attribute value.
	S3Prefix string `alloy:"s3_prefix,attr"`
}

func (args ResourceAttrsToS3) Convert() awss3exporter.ResourceAttrsToS3 {
	return awss3exporter.ResourceAttrsToS3{
		S3Prefix: args.S3Prefix,
	}
}

// S3 Uploader Arguments Block
type S3Uploader struct {
	Region            string                 `alloy:"region,attr,optional"`
	S3Bucket          string                 `alloy:"s3_bucket,attr"`
	S3Prefix          string                 `alloy:"s3_prefix,attr"`
	S3PartitionFormat string                 `alloy:"s3_partition_format,attr,optional"`
	RoleArn           string                 `alloy:"role_arn,attr,optional"`
	FilePrefix        string                 `alloy:"file_prefix,attr,optional"`
	Endpoint          string                 `alloy:"endpoint,attr,optional"`
	S3ForcePathStyle  bool                   `alloy:"s3_force_path_style,attr,optional"`
	DisableSSL        bool                   `alloy:"disable_ssl,attr,optional"`
	Compression       configcompression.Type `alloy:"compression,attr,optional"`
	ACL               string                 `alloy:"acl,attr,optional"`
	StorageClass      string                 `alloy:"storage_class,attr,optional"`
}

func (args *S3Uploader) SetToDefault() {
	*args = S3Uploader{
		Region:            "us-east-1",
		S3ForcePathStyle:  false,
		DisableSSL:        false,
		S3PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
		Compression:       "none",
		StorageClass:      "STANDARD",
	}
}

func (args *S3Uploader) Convert() awss3exporter.S3UploaderConfig {
	return awss3exporter.S3UploaderConfig{
		Region:            args.Region,
		S3Bucket:          args.S3Bucket,
		S3Prefix:          args.S3Prefix,
		S3PartitionFormat: args.S3PartitionFormat,
		FilePrefix:        args.FilePrefix,
		Endpoint:          args.Endpoint,
		RoleArn:           args.RoleArn,
		S3ForcePathStyle:  args.S3ForcePathStyle,
		DisableSSL:        args.DisableSSL,
		Compression:       args.Compression,
		ACL:               args.ACL,
		StorageClass:      args.StorageClass,
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
