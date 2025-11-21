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
	S3Partition         string `alloy:"s3_partition,attr,optional"`
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

type Encoding struct {
	Extension otelcomponent.ID `alloy:"extension,attr"`
	Suffix    string           `alloy:"suffix,attr"`
}

type Notifications struct {
	OpAMP *component.ID `alloy:"opampextension,attr"`
}

type Arguments struct {
	StartTime     string             `alloy:"start_time,attr,optional"`
	EndTime       string             `alloy:"end_time,attr,optional"`
	Encodings     []Encoding         `alloy:"encoding,block,optional"`
	Notifications Notifications      `alloy:"notifications,block,optional"`
	S3Downloader  S3DownloaderConfig `alloy:"s3downloader,block"`
	SQS           *SQSConfig         `alloy:"sqs,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

// SetToDefault implements syntax.Defaulter.
func (*Arguments) SetToDefault() {
	// Defaults filled by upstream OTel receiver in a factory.
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// TODO
	return nil
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	panic("TODO")
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
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}
