package otelcolconvert

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"

	otelconfig "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/splunkhec"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func init() {
	converters = append(converters, splunkhecReceiverConverter{})
}

type splunkhecReceiverConverter struct{}

func (splunkhecReceiverConverter) Factory() component.Factory {
	return splunkhecreceiver.NewFactory()
}

func (splunkhecReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.splunkhec"
}

func (splunkhecReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	block := common.NewBlockWithOverride(
		[]string{"otelcol", "receiver", "splunkhec"},
		state.AlloyComponentLabel(),
		toSplunkhecReceiver(state, id, cfg.(*splunkhecreceiver.Config)),
	)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toSplunkhecReceiver(state *State, id componentstatus.InstanceID, cfg *splunkhecreceiver.Config) *splunkhec.Arguments {
	args := &splunkhec.Arguments{
		HTTPServer: *toHTTPServerArguments(&cfg.ServerConfig),
		RawPath:    cfg.RawPath,
		HealthPath: cfg.HealthPath,
		Splitting:  splunkhec.SplittingStrategy(cfg.Splitting),
		HecToOtelAttrs: splunkhec.HecToOtelAttrsArguments{
			Source:     cfg.HecToOtelAttrs.Source,
			SourceType: cfg.HecToOtelAttrs.SourceType,
			Index:      cfg.HecToOtelAttrs.Index,
			Host:       cfg.HecToOtelAttrs.Host,
		},
		DebugMetrics: common.DefaultValue[otelconfig.DebugMetricsArguments](),
		Output: &splunkhec.ConsumerArguments{
			Metrics: ToTokenizedConsumers(state.Next(id, pipeline.SignalLogs)),
			Logs:    ToTokenizedConsumers(state.Next(id, pipeline.SignalMetrics)),
		},
	}

	return args
}
