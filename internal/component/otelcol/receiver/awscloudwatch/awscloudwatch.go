// Package awscloudwatch provides an otelcol.receiver.awscloudwatch component.
package awscloudwatch

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

// To test this component locally, you can use localstack to run a local aws cloudwatch and create log groups and streams via the aws cli.
// There is no way to override the endpoint for the receiver so you need to use a local version of the Otel component and override it in the code.
func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.awscloudwatch",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := awscloudwatchreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.awscloudwatch component.
type Arguments struct {
	Region       string                      `alloy:"region,attr"`
	Profile      string                      `alloy:"profile,attr,optional"`
	IMDSEndpoint string                      `alloy:"imds_endpoint,attr,optional"`
	Logs         LogsConfig                  `alloy:"logs,block,optional"`
	Storage      *extension.ExtensionHandler `alloy:"storage,attr,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Logs.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	otelConfig, err := args.Convert()
	if err != nil {
		return err
	}

	return otelConfig.(*awscloudwatchreceiver.Config).Validate()
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	otelConfig := &awscloudwatchreceiver.Config{
		Region:       args.Region,
		Profile:      args.Profile,
		IMDSEndpoint: args.IMDSEndpoint,
		Logs:         args.Logs.Convert(),
	}

	// If no autodiscover or named configs are provided, set the autodiscover config with a default limit.
	if args.Logs.Groups.AutodiscoverConfig == nil && len(args.Logs.Groups.NamedConfigs) == 0 {
		otelConfig.Logs.Groups.AutodiscoverConfig = &awscloudwatchreceiver.AutodiscoverConfig{
			Limit: defaultLogGroupLimit,
		}
	}

	// If the autodiscover config is provided but the limit is not set, set the limit to the default.
	if args.Logs.Groups.AutodiscoverConfig != nil && args.Logs.Groups.AutodiscoverConfig.Limit == nil {
		otelConfig.Logs.Groups.AutodiscoverConfig.Limit = defaultLogGroupLimit
	}

	// Configure storage if args.Storage is set.
	if args.Storage != nil {
		if args.Storage.Extension == nil {
			return nil, fmt.Errorf("missing storage extension")
		}

		otelConfig.StorageID = &args.Storage.ID
	}

	return otelConfig, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.Storage != nil {
		m[args.Storage.ID] = args.Storage.Extension
	}
	return m
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
