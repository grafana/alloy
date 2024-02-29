// Package tail_sampling provides an otelcol.processor.tail_sampling component.
package tail_sampling

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor"
	"github.com/grafana/agent/internal/featuregate"
	tsp "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.tail_sampling",
		Stability: featuregate.StabilityBeta,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := tsp.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.tail_sampling component.
type Arguments struct {
	PolicyCfgs              []PolicyConfig `river:"policy,block"`
	DecisionWait            time.Duration  `river:"decision_wait,attr,optional"`
	NumTraces               uint64         `river:"num_traces,attr,optional"`
	ExpectedNewTracesPerSec uint64         `river:"expected_new_traces_per_sec,attr,optional"`
	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var (
	_ processor.Arguments = Arguments{}
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	DecisionWait:            30 * time.Second,
	NumTraces:               50000,
	ExpectedNewTracesPerSec: 0,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	if args.DecisionWait.Milliseconds() <= 0 {
		return fmt.Errorf("decision_wait must be greater than zero")
	}

	if args.NumTraces <= 0 {
		return fmt.Errorf("num_traces must be greater than zero")
	}

	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	var otelPolicyCfgs []tsp.PolicyCfg
	for _, policyCfg := range args.PolicyCfgs {
		otelPolicyCfgs = append(otelPolicyCfgs, policyCfg.Convert())
	}

	return &tsp.Config{
		DecisionWait:            args.DecisionWait,
		NumTraces:               args.NumTraces,
		ExpectedNewTracesPerSec: args.ExpectedNewTracesPerSec,
		PolicyCfgs:              otelPolicyCfgs,
	}, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}
