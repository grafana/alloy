// Package signaltometrics provides an otelcol.connector.signaltometrics component.
package signaltometrics

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.signaltometrics",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := signaltometricsconnector.NewFactory()
			return connector.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.signaltometrics component.
type Arguments struct {
	// Spans defines metrics generated from spans.
	Spans []MetricInfo `alloy:"spans,block,optional"`
	// Datapoints defines metrics generated from metric data points.
	Datapoints []MetricInfo `alloy:"datapoints,block,optional"`
	// Logs defines metrics generated from log records.
	Logs []MetricInfo `alloy:"logs,block,optional"`

	// ErrorMode determines how the connector reacts to errors that occur while
	// processing an OTTL condition or statement during runtime data consumption.
	ErrorMode ottl.ErrorMode `alloy:"error_mode,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ syntax.Validator    = (*Arguments)(nil)
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ connector.Arguments = (*Arguments)(nil)
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	ErrorMode: ottl.PropagateError,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	return args.convertImpl().Validate()
}

// Convert implements connector.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return args.convertImpl(), nil
}

// convertImpl returns the concrete upstream config type so it can be reused by
// both Convert and Validate.
func (args Arguments) convertImpl() *config.Config {
	return &config.Config{
		Spans:      convertMetricInfos(args.Spans),
		Datapoints: convertMetricInfos(args.Datapoints),
		Logs:       convertMetricInfos(args.Logs),
		ErrorMode:  args.ErrorMode,
	}
}

// Extensions implements connector.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements connector.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ConnectorType implements connector.Arguments.
func (Arguments) ConnectorType() int {
	return connector.ConnectorTracesToMetrics | connector.ConnectorMetricsToMetrics | connector.ConnectorLogsToMetrics
}

// DebugMetricsConfig implements connector.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
