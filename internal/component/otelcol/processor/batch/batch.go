// Package batch provides an otelcol.processor.batch component.
package batch

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/batchprocessor"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.batch",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := batchprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.batch component.
type Arguments struct {
	Timeout                  time.Duration `alloy:"timeout,attr,optional"`
	SendBatchSize            uint32        `alloy:"send_batch_size,attr,optional"`
	SendBatchMaxSize         uint32        `alloy:"send_batch_max_size,attr,optional"`
	MetadataKeys             []string      `alloy:"metadata_keys,attr,optional"`
	MetadataCardinalityLimit uint32        `alloy:"metadata_cardinality_limit,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = Arguments{}
)

// DefaultArguments holds default settings for Arguments, derived from upstream.
var DefaultArguments = func() Arguments {
	var a Arguments
	a.SetToDefault()
	return a
}()

// SetToDefault implements syntax.Defaulter. Panics on error because
// SetToDefault can't return one.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.DebugMetrics.SetToDefault()
	if err := otelcol.DefaultsFrom(batchprocessor.NewFactory().CreateDefaultConfig(), args); err != nil {
		panic(err)
	}
	// Declared divergences from upstream, kept for Grafana Cloud batching history.
	args.SendBatchSize = 2000
	args.SendBatchMaxSize = 3000
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.SendBatchMaxSize > 0 && args.SendBatchMaxSize < args.SendBatchSize {
		return fmt.Errorf("send_batch_max_size must be greater or equal to send_batch_size when not 0")
	}
	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return otelcol.ConvertViaConfmap(args, batchprocessor.NewFactory(),
		otelcol.WithConvertSkip("output", "debug_metrics"),
	)
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements processor.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
