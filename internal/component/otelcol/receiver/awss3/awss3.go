package awss3

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

var (
	_ receiver.Arguments = Arguments{}
	_ syntax.Defaulter   = (*Arguments)(nil)
	_ syntax.Validator   = (*Arguments)(nil)
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.awss3",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := awss3receiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

type S3DownloaderConfig struct {
	Region              string `alloy:"region,attr,optional"`
	S3Bucket            string `alloy:"s3_bucket,attr"`
	S3Prefix            string `alloy:"s3_prefix,attr"`
	S3PartitionFormat   string `alloy:"s3_partition_format,attr,optional"`
	S3PartitionTimezone string `alloy:"s3_partition_timezone,attr,optional"`
	FilePrefix          string `alloy:"file_prefix,attr,optional"`
	Endpoint            string `alloy:"endpoint,attr,optional"`
	EndpointPartitionID string `alloy:"endpoint_partition_id,attr,optional"`
	S3ForcePathStyle    bool   `alloy:"s3_force_path_style,attr,optional"`
}

type SQSConfig struct {
	QueueURL            string `alloy:"queue_url,attr,optional"`
	Region              string `alloy:"region,attr,optional"`
	Endpoint            string `alloy:"endpoint,attr,optional"`
	WaitTimeSeconds     *int64 `alloy:"wait_time_seconds,attr,optional"`
	MaxNumberOfMessages *int64 `alloy:"max_number_of_messages,attr,optional"`
}

type Arguments struct {
	StartTime    string             `alloy:"start_time,attr,optional"`
	EndTime      string             `alloy:"end_time,attr,optional"`
	S3Downloader S3DownloaderConfig `alloy:"s3downloader,block"`
	SQS          *SQSConfig         `alloy:"sqs,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// ArgumentsFromConfig constructs component arguments from awss3receiver config.
func ArgumentsFromConfig(cfg *awss3receiver.Config) Arguments {
	// TODO(x1unix): map Encodings
	args := Arguments{
		StartTime: cfg.StartTime,
		EndTime:   cfg.EndTime,
		S3Downloader: S3DownloaderConfig{
			Region:              cfg.S3Downloader.Region,
			S3Bucket:            cfg.S3Downloader.S3Bucket,
			S3Prefix:            cfg.S3Downloader.S3Prefix,
			S3PartitionFormat:   cfg.S3Downloader.S3PartitionFormat,
			S3PartitionTimezone: cfg.S3Downloader.S3PartitionTimezone,
			FilePrefix:          cfg.S3Downloader.FilePrefix,
			Endpoint:            cfg.S3Downloader.Endpoint,
			EndpointPartitionID: cfg.S3Downloader.EndpointPartitionID,
			S3ForcePathStyle:    cfg.S3Downloader.S3ForcePathStyle,
		},
	}

	if cfg.SQS != nil {
		args.SQS = &SQSConfig{
			QueueURL:            cfg.SQS.QueueURL,
			Region:              cfg.SQS.Region,
			Endpoint:            cfg.SQS.Endpoint,
			WaitTimeSeconds:     cfg.SQS.WaitTimeSeconds,
			MaxNumberOfMessages: cfg.SQS.MaxNumberOfMessages,
		}
	}

	return args
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	defaultCfg := awss3receiver.NewFactory().CreateDefaultConfig().(*awss3receiver.Config)
	*args = ArgumentsFromConfig(defaultCfg)
}

func (args Arguments) receiverConfig() *awss3receiver.Config {
	// TODO(x1unix): map Encodings and Notification with components.
	cfg := &awss3receiver.Config{
		StartTime: args.StartTime,
		EndTime:   args.EndTime,
		S3Downloader: awss3receiver.S3DownloaderConfig{
			Region:              args.S3Downloader.Region,
			S3Bucket:            args.S3Downloader.S3Bucket,
			S3Prefix:            args.S3Downloader.S3Prefix,
			S3PartitionFormat:   args.S3Downloader.S3PartitionFormat,
			S3PartitionTimezone: args.S3Downloader.S3PartitionTimezone,
			FilePrefix:          args.S3Downloader.FilePrefix,
			Endpoint:            args.S3Downloader.Endpoint,
			EndpointPartitionID: args.S3Downloader.EndpointPartitionID,
			S3ForcePathStyle:    args.S3Downloader.S3ForcePathStyle,
		},
	}

	if args.SQS != nil {
		cfg.SQS = &awss3receiver.SQSConfig{
			QueueURL:            args.SQS.QueueURL,
			Region:              args.SQS.Region,
			Endpoint:            args.SQS.Endpoint,
			WaitTimeSeconds:     args.SQS.WaitTimeSeconds,
			MaxNumberOfMessages: args.SQS.MaxNumberOfMessages,
		}
	}

	return cfg
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return args.receiverConfig(), nil
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	otelCfg := args.receiverConfig()
	return otelCfg.Validate()
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	// Underlying receiver doesn't support debug metrics.
	// Return defaults (see: DebugMetricsArguments.SetToDefault)
	return otelcolCfg.DebugMetricsArguments{
		DisableHighCardinalityMetrics: true,
		Level:                         otelcolCfg.LevelDetailed,
	}
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	// TODO(x1unix): expose components after Encodings will be exposed (See: #4938 and #4934)
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}
