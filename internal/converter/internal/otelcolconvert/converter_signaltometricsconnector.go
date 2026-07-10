package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/connector/signaltometrics"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, signalToMetricsConnectorConverter{})
}

type signalToMetricsConnectorConverter struct{}

func (signalToMetricsConnectorConverter) Factory() component.Factory {
	return signaltometricsconnector.NewFactory()
}

func (signalToMetricsConnectorConverter) InputComponentName() string {
	return "otelcol.connector.signaltometrics"
}

func (signalToMetricsConnectorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toSignalToMetricsConnector(state, id, cfg.(*config.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "connector", "signaltometrics"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toSignalToMetricsConnector(state *State, id componentstatus.InstanceID, cfg *config.Config) *signaltometrics.Arguments {
	if cfg == nil {
		return nil
	}

	nextMetrics := state.Next(id, pipeline.SignalMetrics)

	return &signaltometrics.Arguments{
		Spans:      toSignalToMetricsMetricInfos(cfg.Spans),
		Datapoints: toSignalToMetricsMetricInfos(cfg.Datapoints),
		Logs:       toSignalToMetricsMetricInfos(cfg.Logs),
		ErrorMode:  cfg.ErrorMode,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},

		DebugMetrics: common.DefaultValue[signaltometrics.Arguments]().DebugMetrics,
	}
}

func toSignalToMetricsMetricInfos(infos []config.MetricInfo) []signaltometrics.MetricInfo {
	if len(infos) == 0 {
		return nil
	}
	res := make([]signaltometrics.MetricInfo, 0, len(infos))
	for _, info := range infos {
		res = append(res, toSignalToMetricsMetricInfo(info))
	}
	return res
}

func toSignalToMetricsMetricInfo(info config.MetricInfo) signaltometrics.MetricInfo {
	res := signaltometrics.MetricInfo{
		Name:                      info.Name,
		Description:               info.Description,
		Unit:                      info.Unit,
		IncludeResourceAttributes: toSignalToMetricsAttributes(info.IncludeResourceAttributes),
		Attributes:                toSignalToMetricsAttributes(info.Attributes),
		Conditions:                info.Conditions,
	}

	if info.Histogram.HasValue() {
		h := info.Histogram.Get()
		res.Histogram = &signaltometrics.Histogram{
			Buckets: h.Buckets,
			Count:   h.Count,
			Value:   h.Value,
		}
	}
	if info.ExponentialHistogram.HasValue() {
		eh := info.ExponentialHistogram.Get()
		res.ExponentialHistogram = &signaltometrics.ExponentialHistogram{
			MaxSize: eh.MaxSize,
			Count:   eh.Count,
			Value:   eh.Value,
		}
	}
	if info.Sum.HasValue() {
		res.Sum = &signaltometrics.Sum{
			Value: info.Sum.Get().Value,
		}
	}
	if info.Gauge.HasValue() {
		res.Gauge = &signaltometrics.Gauge{
			Value: info.Gauge.Get().Value,
		}
	}

	return res
}

func toSignalToMetricsAttributes(attrs []config.Attribute) []signaltometrics.Attribute {
	if len(attrs) == 0 {
		return nil
	}
	res := make([]signaltometrics.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		res = append(res, signaltometrics.Attribute{
			Key:          attr.Key,
			Optional:     attr.Optional,
			DefaultValue: attr.DefaultValue,
		})
	}
	return res
}
