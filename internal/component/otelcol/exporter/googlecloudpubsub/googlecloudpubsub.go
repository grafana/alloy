package googlecloudpubsub

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	googlecloudpubsubconfig "github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloudpubsub/config"
	"github.com/grafana/alloy/syntax"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.googlecloudpubsub",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			f := googlecloudpubsubexporter.NewFactory()
			return exporter.New(opts, f, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

type Arguments struct {
	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// Google Cloud Pub/Sub specific configuration settings
	Project     string                                                           `alloy:"project,attr,optional"`
	UserAgent   string                                                           `alloy:"user_agent,attr,optional"`
	Topic       string                                                           `alloy:"topic,attr"`
	Compression string                                                           `alloy:"compression,attr,optional"`
	Watermark   googlecloudpubsubconfig.GoogleCloudPubSubWatermarkArguments      `alloy:"watermark,block,optional"`
	Endpoint    string                                                           `alloy:"endpoint,attr,optional"`
	Insecure    bool                                                             `alloy:"insecure,attr,optional"`
	Ordering    googlecloudpubsubconfig.GoogleCloudPubSubOrderingConfigArguments `alloy:"ordering,block,optional"`
	Timeout     time.Duration                                                    `alloy:"timeout,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// Compile time assertions
var (
	_ exporter.Arguments = Arguments{}
	_ syntax.Defaulter   = &Arguments{}
	_ syntax.Validator   = &Arguments{}
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()

	args.UserAgent = "opentelemetry-collector-contrib {{version}}"
	args.Watermark.SetToDefault()
	args.Ordering.SetToDefault()
	args.Timeout = time.Second * 12

	args.DebugMetrics.SetToDefault()
}

func (args Arguments) Convert() (otelcomponent.Config, error) {
	var result googlecloudpubsubexporter.Config

	result.BackOffConfig = *args.Retry.Convert()

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	result.QueueSettings = q

	result.ProjectID = args.Project
	result.UserAgent = args.UserAgent
	result.Topic = args.Topic
	result.Compression = args.Compression
	result.Watermark = args.Watermark.Convert()
	result.Endpoint = args.Endpoint
	result.Insecure = args.Insecure
	result.Ordering = args.Ordering.Convert()
	result.TimeoutSettings.Timeout = args.Timeout

	return &result, nil
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
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}

	googleCloudPubSubCfg := otelCfg.(*googlecloudpubsubexporter.Config)
	return googleCloudPubSubCfg.Validate()
}
