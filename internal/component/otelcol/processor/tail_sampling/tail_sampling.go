// Package tail_sampling provides an otelcol.processor.tail_sampling component.
package tail_sampling

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	tsp "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.tail_sampling",
		Stability: featuregate.StabilityGenerallyAvailable,
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
	PolicyCfgs                    []PolicyConfig      `alloy:"policy,block"`
	DecisionWait                  time.Duration       `alloy:"decision_wait,attr,optional"`
	DecisionWaitAfterRootReceived time.Duration       `alloy:"decision_wait_after_root_received,attr,optional"`
	NumTraces                     uint64              `alloy:"num_traces,attr,optional"`
	BlockOnOverflow               bool                `alloy:"block_on_overflow,attr,optional"`
	ExpectedNewTracesPerSec       uint64              `alloy:"expected_new_traces_per_sec,attr,optional"`
	SampleOnFirstMatch            bool                `alloy:"sample_on_first_match,attr,optional"`
	DropPendingTracesOnShutdown   bool                `alloy:"drop_pending_traces_on_shutdown,attr,optional"`
	MaximumTraceSizeBytes         uint64              `alloy:"maximum_trace_size_bytes,attr,optional"`
	DecisionCache                 DecisionCacheConfig `alloy:"decision_cache,attr,optional"`
	// SamplingStrategy controls how/when sampling decisions are made.
	// Valid values: "trace-complete" (default) or "span-ingest".
	SamplingStrategy string `alloy:"sampling_strategy,attr,optional"`
	// TailStorage configures an optional extension for buffering spans on disk.
	TailStorage *extension.ExtensionHandler `alloy:"tail_storage,attr,optional"`
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
	DecisionWait:            30 * time.Second,
	NumTraces:               50000,
	ExpectedNewTracesPerSec: 0,
	SamplingStrategy:        "trace-complete",
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
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

	result := &tsp.Config{
		DecisionWait:                  args.DecisionWait,
		DecisionWaitAfterRootReceived: args.DecisionWaitAfterRootReceived,
		NumTraces:                     args.NumTraces,
		BlockOnOverflow:               args.BlockOnOverflow,
		ExpectedNewTracesPerSec:       args.ExpectedNewTracesPerSec,
		SampleOnFirstMatch:            args.SampleOnFirstMatch,
		DropPendingTracesOnShutdown:   args.DropPendingTracesOnShutdown,
		MaximumTraceSizeBytes:         args.MaximumTraceSizeBytes,
		PolicyCfgs:                    otelPolicyCfgs,
		DecisionCache:                 args.DecisionCache.Convert(),
	}

	// SamplingStrategy uses an unexported type; use mapstructure to set it.
	if args.SamplingStrategy != "" {
		if err := mapstructure.Decode(map[string]any{"sampling_strategy": args.SamplingStrategy}, result); err != nil {
			return nil, fmt.Errorf("invalid sampling_strategy: %w", err)
		}
	}

	if args.TailStorage != nil {
		if args.TailStorage.Extension == nil {
			return nil, fmt.Errorf("missing tail_storage extension")
		}
		result.TailStorageID = &args.TailStorage.ID
	}

	return result, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if args.TailStorage != nil {
		m[args.TailStorage.ID] = args.TailStorage.Extension
	}
	return m
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
