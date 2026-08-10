package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/cardinalityguardian"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, cardinalityguardianProcessorConverter{})
}

type cardinalityguardianProcessorConverter struct{}

func (cardinalityguardianProcessorConverter) Factory() component.Factory {
	return cardinalityguardianprocessor.NewFactory()
}

func (cardinalityguardianProcessorConverter) InputComponentName() string {
	return "otelcol.processor.cardinality_guardian"
}

func (cardinalityguardianProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toCardinalityguardianProcessor(state, id, cfg.(*cardinalityguardianprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "cardinality_guardian"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toCardinalityguardianProcessor(state *State, id componentstatus.InstanceID, cfg *cardinalityguardianprocessor.Config) *cardinalityguardian.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
	)

	return &cardinalityguardian.Arguments{
		MaxCardinalityDeltaPerEpoch: cfg.MaxCardinalityDeltaPerEpoch,
		EpochDurationSeconds:        cfg.EpochDurationSeconds,
		NeverDropLabels:             cfg.NeverDropLabels,
		EnforcementMode:             string(cfg.EnforcementMode),
		EstimatedCostPerMetricMonth: cfg.EstimatedCostPerMetricMonth,
		TopOffendersCount:           cfg.TopOffendersCount,
		MaxTrackerCount:             cfg.MaxTrackerCount,
		MetricOverrides:             cfg.MetricOverrides,
		DropLogMaxPerEpoch:          cfg.DropLogMaxPerEpoch,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},
		DebugMetrics: common.DefaultValue[cardinalityguardian.Arguments]().DebugMetrics,
	}
}
