// Package googlecloud provides an otelcol.exporter.googlecloud component
package googlecloud

import (
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	googlecloudconfig "github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloud/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.googlecloud",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := googlecloudexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

// Arguments configures the otelcol.exporter.googlecloud component.
type Arguments struct {
	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`

	// Google Cloud specific configuration settings
	Project                 string                                            `alloy:"project,attr,optional"`
	DestinationProjectQuota bool                                              `alloy:"destination_project_quota,attr,optional"`
	UserAgent               string                                            `alloy:"user_agent,attr,optional"`
	Impersonate             googlecloudconfig.GoogleCloudImpersonateArguments `alloy:"impersonate,block,optional"`
	Metric                  googlecloudconfig.GoogleCloudMetricArguments      `alloy:"metric,block,optional"`
	Trace                   googlecloudconfig.GoogleCloudTraceArguments       `alloy:"trace,block,optional"`
	Log                     googlecloudconfig.GoogleCloudLogArguments         `alloy:"log,block,optional"`

	// DebugMetrics configures component internal metrics. Optional
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Queue.SetToDefault()

	args.UserAgent = "opentelemetry-collector-contrib {{version}}"
	args.Impersonate.SetToDefault()
	args.Metric.SetToDefault()
	args.Trace.SetToDefault()
	args.Log.SetToDefault()

	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	var result googlecloudexporter.Config
	// We need to assign default values first to populate fields with unexported functions
	result.Config = collector.DefaultConfig()

	result.TimeoutSettings.Timeout = 12 * time.Second // https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/70d8986fa3a30e1f26927abffe1880345e2afa3f/exporter/googlecloudexporter/factory.go#L48
	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	result.QueueSettings = q

	result.ProjectID = args.Project
	result.DestinationProjectQuota = args.DestinationProjectQuota
	result.UserAgent = args.UserAgent

	result.ImpersonateConfig = args.Impersonate.Convert(result.ImpersonateConfig)
	result.MetricConfig = args.Metric.Convert(result.MetricConfig)
	result.TraceConfig = args.Trace.Convert(result.TraceConfig)
	result.LogConfig = args.Log.Convert(result.LogConfig)

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
	googlecloudCfg := otelCfg.(*googlecloudexporter.Config)
	return googlecloudCfg.Validate()
}
