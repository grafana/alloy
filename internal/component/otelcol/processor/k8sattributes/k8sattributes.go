// Package attributes provides an otelcol.processor.k8sattributes component.
package k8sattributes

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.k8sattributes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := k8sattributesprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

var (
	_ processor.Arguments = Arguments{}
)

// Arguments configures the otelcol.processor.k8sattributes component.
type Arguments struct {
	AuthType        string              `alloy:"auth_type,attr,optional"`
	Passthrough     bool                `alloy:"passthrough,attr,optional"`
	ExtractConfig   ExtractConfig       `alloy:"extract,block,optional"`
	Filter          FilterConfig        `alloy:"filter,block,optional"`
	PodAssociations PodAssociationSlice `alloy:"pod_association,block,optional"`
	Exclude         ExcludeConfig       `alloy:"exclude,block,optional"`

	// Determines if the processor should wait k8s metadata to be synced when starting.
	WaitForMetadata bool `alloy:"wait_for_metadata,attr,optional"`

	// The maximum time the processor will wait for the k8s metadata to be synced.
	WaitForMetadataTimeout time.Duration `alloy:"wait_for_metadata_timeout,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	// These are default excludes from upstream opentelemetry-collector-contrib
	// Source: https://github.com/open-telemetry/opentelemetry-collector-contrib/blame/main/processor/k8sattributesprocessor/factory.go#L21
	args.Exclude = ExcludeConfig{
		Pods: []ExcludePodConfig{
			{Name: "jaeger-agent"},
			{Name: "jaeger-collector"},
		},
	}
	args.WaitForMetadataTimeout = 10 * time.Second
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	cfg, err := args.Convert()
	if err != nil {
		return err
	}

	return cfg.(*k8sattributesprocessor.Config).Validate()
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]any)

	if args.AuthType == "" {
		input["auth_type"] = "serviceAccount"
	} else {
		input["auth_type"] = args.AuthType
	}

	input["wait_for_metadata"] = args.WaitForMetadata

	input["passthrough"] = args.Passthrough

	if extract := args.ExtractConfig.convert(); len(extract) > 0 {
		input["extract"] = extract
	}

	if filter := args.Filter.convert(); len(filter) > 0 {
		input["filter"] = filter
	}

	if podAssociations := args.PodAssociations.convert(); len(podAssociations) > 0 {
		input["pod_association"] = podAssociations
	}

	if exclude := args.Exclude.convert(); len(exclude) > 0 {
		input["exclude"] = exclude
	}

	var result k8sattributesprocessor.Config
	err := mapstructure.Decode(input, &result)

	if err != nil {
		return nil, err
	}

	// Set the timeout after the decoding step.
	// That way we don't have to convert a duration to a string.
	result.WaitForMetadataTimeout = args.WaitForMetadataTimeout

	return &result, nil
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
