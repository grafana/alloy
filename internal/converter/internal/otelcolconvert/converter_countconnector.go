package otelcolconvert

import (
	"fmt"
	"sort"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/connector/count"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, countConnectorConverter{})
}

type countConnectorConverter struct{}

func (countConnectorConverter) Factory() component.Factory {
	return countconnector.NewFactory()
}

func (countConnectorConverter) InputComponentName() string {
	return "otelcol.connector.count"
}

func (countConnectorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toCountConnector(state, id, cfg.(*countconnector.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "connector", "count"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toCountConnector(state *State, id componentstatus.InstanceID, cfg *countconnector.Config) *count.Arguments {
	if cfg == nil {
		return nil
	}
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
	)

	// Convert spans (sorted by name for deterministic output)
	var spans []count.MetricInfo
	spanNames := make([]string, 0, len(cfg.Spans))
	for name := range cfg.Spans {
		spanNames = append(spanNames, name)
	}
	sort.Strings(spanNames)
	for _, name := range spanNames {
		spans = append(spans, toCountMetricInfo(name, cfg.Spans[name]))
	}

	// Convert spanevents (sorted by name for deterministic output)
	var spanEvents []count.MetricInfo
	spanEventNames := make([]string, 0, len(cfg.SpanEvents))
	for name := range cfg.SpanEvents {
		spanEventNames = append(spanEventNames, name)
	}
	sort.Strings(spanEventNames)
	for _, name := range spanEventNames {
		spanEvents = append(spanEvents, toCountMetricInfo(name, cfg.SpanEvents[name]))
	}

	// Convert metrics (sorted by name for deterministic output)
	var metrics []count.MetricInfo
	metricNames := make([]string, 0, len(cfg.Metrics))
	for name := range cfg.Metrics {
		metricNames = append(metricNames, name)
	}
	sort.Strings(metricNames)
	for _, name := range metricNames {
		metrics = append(metrics, toCountMetricInfo(name, cfg.Metrics[name]))
	}

	// Convert datapoints (sorted by name for deterministic output)
	var dataPoints []count.MetricInfo
	dataPointNames := make([]string, 0, len(cfg.DataPoints))
	for name := range cfg.DataPoints {
		dataPointNames = append(dataPointNames, name)
	}
	sort.Strings(dataPointNames)
	for _, name := range dataPointNames {
		dataPoints = append(dataPoints, toCountMetricInfo(name, cfg.DataPoints[name]))
	}

	// Convert logs (sorted by name for deterministic output)
	var logs []count.MetricInfo
	logNames := make([]string, 0, len(cfg.Logs))
	for name := range cfg.Logs {
		logNames = append(logNames, name)
	}
	sort.Strings(logNames)
	for _, name := range logNames {
		logs = append(logs, toCountMetricInfo(name, cfg.Logs[name]))
	}

	return &count.Arguments{
		Spans:      spans,
		SpanEvents: spanEvents,
		Metrics:    metrics,
		DataPoints: dataPoints,
		Logs:       logs,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},

		DebugMetrics: common.DefaultValue[count.Arguments]().DebugMetrics,
	}
}

func toCountMetricInfo(name string, info countconnector.MetricInfo) count.MetricInfo {
	var attributes []count.AttributeConfig
	for _, attr := range info.Attributes {
		attributes = append(attributes, count.AttributeConfig{
			Key:          attr.Key,
			DefaultValue: attr.DefaultValue,
		})
	}

	return count.MetricInfo{
		Name:        name,
		Description: info.Description,
		Conditions:  info.Conditions,
		Attributes:  attributes,
	}
}
