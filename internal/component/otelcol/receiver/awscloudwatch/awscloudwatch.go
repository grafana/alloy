// Package awscloudwatch provides an otelcol.receiver.awscloudwatch component.
package awscloudwatch

import (
	"fmt"
	"net/url"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
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
	Region       string      `alloy:"region,attr"`
	Profile      string      `alloy:"profile,attr,optional"`
	IMDSEndpoint string      `alloy:"imds_endpoint,attr,optional"`
	Logs         *LogsConfig `alloy:"logs,block,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// The defaultLogGroupLimit is not set in the SetToDefault but in the Validate method because
// the block that contains it is optional.
var defaultLogGroupLimit = 50

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Logs = &LogsConfig{}
	args.Logs.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.IMDSEndpoint != "" {
		_, err := url.ParseRequestURI(args.IMDSEndpoint)
		if err != nil {
			return fmt.Errorf("unable to parse URI for imds_endpoint: %w", err)
		}
	}

	if args.Logs == nil {
		return fmt.Errorf("logs must be configured")
	}

	if args.Logs.MaxEventsPerRequest <= 0 {
		return fmt.Errorf("max_events_per_request must be greater than 0")
	}

	if args.Logs.PollInterval < time.Second {
		return fmt.Errorf("poll_interval must be greater than 1 second")
	}

	if args.Logs.Groups.AutodiscoverConfig != nil && len(args.Logs.Groups.NamedConfigs) > 0 {
		return fmt.Errorf("autodiscover and named configs cannot be configured at the same time")
	}

	if args.Logs.Groups.AutodiscoverConfig != nil {
		if args.Logs.Groups.AutodiscoverConfig.Limit == nil {
			args.Logs.Groups.AutodiscoverConfig.Limit = &defaultLogGroupLimit

			if *args.Logs.Groups.AutodiscoverConfig.Limit <= 0 {
				return fmt.Errorf("autodiscover limit must be greater than 0")
			}
		}
	} else if len(args.Logs.Groups.NamedConfigs) == 0 {
		args.Logs.Groups.AutodiscoverConfig = &AutodiscoverConfig{
			Limit: &defaultLogGroupLimit,
		}
	}

	return nil
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &awscloudwatchreceiver.Config{
		Region:       args.Region,
		Profile:      args.Profile,
		IMDSEndpoint: args.IMDSEndpoint,
		Logs:         args.Logs.Convert(),
	}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
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
