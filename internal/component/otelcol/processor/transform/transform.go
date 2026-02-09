// Package transform provides an otelcol.processor.transform component.
package transform

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.transform",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := transformprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

type ContextID string

const (
	Resource  ContextID = "resource"
	Scope     ContextID = "scope"
	Span      ContextID = "span"
	SpanEvent ContextID = "spanevent"
	Metric    ContextID = "metric"
	DataPoint ContextID = "datapoint"
	Log       ContextID = "log"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case Resource, Scope, Span, SpanEvent, Metric, DataPoint, Log:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %v", str)
	}
}

type Statements []string

type ContextStatementsSlice []ContextStatements

type ContextStatements struct {
	Context    ContextID      `alloy:"context,attr"`
	Conditions []string       `alloy:"conditions,attr,optional"`
	Statements Statements     `alloy:"statements,attr"`
	ErrorMode  ottl.ErrorMode `alloy:"error_mode,attr,optional"`
}

type NoContextStatements struct {
	Trace  Statements `alloy:"trace,attr,optional"`
	Metric Statements `alloy:"metric,attr,optional"`
	Log    Statements `alloy:"log,attr,optional"`
}

// Arguments configures the otelcol.processor.transform component.
type Arguments struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing a statement.
	ErrorMode ottl.ErrorMode `alloy:"error_mode,attr,optional"`

	Statements NoContextStatements `alloy:"statements,block,optional"`

	TraceStatements  ContextStatementsSlice `alloy:"trace_statements,block,optional"`
	MetricStatements ContextStatementsSlice `alloy:"metric_statements,block,optional"`
	LogStatements    ContextStatementsSlice `alloy:"log_statements,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = Arguments{}
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
	otelArgs, err := args.convertImpl()
	if err != nil {
		return err
	}
	return otelArgs.Validate()
}

func convertNoContext(stmts Statements) ContextStatementsSlice {
	if stmts == nil {
		return nil
	}

	return ContextStatementsSlice{
		{
			Context:    "",
			Statements: stmts,
		},
	}
}

func (stmts *ContextStatementsSlice) convert() []any {
	if stmts == nil {
		return nil
	}

	res := make([]any, 0, len(*stmts))

	if len(*stmts) == 0 {
		return res
	}

	for _, stmt := range *stmts {
		res = append(res, stmt.convert())
	}
	return res
}

func (args *ContextStatements) convert() map[string]any {
	if args == nil {
		return nil
	}

	return map[string]any{
		"context":    args.Context,
		"statements": args.Statements,
		"conditions": args.Conditions,
		"error_mode": args.ErrorMode,
	}
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return args.convertImpl()
}

// convertImpl is a helper function which returns the real type of the config,
// instead of the otelcomponent.Config interface.
func (args Arguments) convertImpl() (*transformprocessor.Config, error) {
	input := make(map[string]any)

	input["error_mode"] = args.ErrorMode

	args.TraceStatements = append(args.TraceStatements, convertNoContext(args.Statements.Trace)...)
	if len(args.TraceStatements) > 0 {
		input["trace_statements"] = args.TraceStatements.convert()
	}

	args.MetricStatements = append(args.MetricStatements, convertNoContext(args.Statements.Metric)...)
	if len(args.MetricStatements) > 0 {
		input["metric_statements"] = args.MetricStatements.convert()
	}

	args.LogStatements = append(args.LogStatements, convertNoContext(args.Statements.Log)...)
	if len(args.LogStatements) > 0 {
		input["log_statements"] = args.LogStatements.convert()
	}

	cfg := transformprocessor.NewFactory().CreateDefaultConfig().(*transformprocessor.Config)
	err := mapstructure.Decode(input, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
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
