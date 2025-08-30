package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/solace"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, solaceReceiverConverter{})
}

type solaceReceiverConverter struct{}

func (solaceReceiverConverter) Factory() component.Factory { return solacereceiver.NewFactory() }

func (solaceReceiverConverter) InputComponentName() string { return "" }

func (solaceReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toSolaceReceiver(state, id, cfg.(*solacereceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "solace"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toSolaceReceiver(state *State, id componentstatus.InstanceID, cfg *solacereceiver.Config) *solace.Arguments {
	nextTraces := state.Next(id, pipeline.SignalTraces)

	var broker string
	if len(cfg.Broker) == 0 {
		broker = ""
	} else {
		broker = cfg.Broker[0]
	}

	return &solace.Arguments{
		Broker:     broker,
		Queue:      cfg.Queue,
		MaxUnacked: cfg.MaxUnacked,

		Auth:         toSolaceAuthentication(cfg.Auth),
		TLS:          toTLSClientArguments(cfg.TLS),
		Flow:         toSolaceFlow(cfg.Flow),
		DebugMetrics: common.DefaultValue[solace.Arguments]().DebugMetrics,
		Output: &otelcol.ConsumerArguments{
			Traces: ToTokenizedConsumers(nextTraces),
		},
	}
}

func toSolaceAuthentication(cfg solacereceiver.Authentication) solace.Authentication {
	return solace.Authentication{
		PlainText: toSaslPlaintext(cfg.PlainText),
		XAuth2:    toSaslXAuth2(cfg.XAuth2),
		External:  toSaslExternal(cfg.External),
	}
}

func toSaslPlaintext(cfg configoptional.Optional[solacereceiver.SaslPlainTextConfig]) *solace.SaslPlainTextConfig {
	if !cfg.HasValue() {
		return nil
	}

	return &solace.SaslPlainTextConfig{
		Username: cfg.Get().Username,
		Password: alloytypes.Secret(cfg.Get().Password),
	}
}

func toSaslXAuth2(cfg configoptional.Optional[solacereceiver.SaslXAuth2Config]) *solace.SaslXAuth2Config {
	if !cfg.HasValue() {
		return nil
	}

	return &solace.SaslXAuth2Config{
		Username: cfg.Get().Username,
		Bearer:   cfg.Get().Bearer,
	}
}

func toSaslExternal(cfg configoptional.Optional[solacereceiver.SaslExternalConfig]) *solace.SaslExternalConfig {
	if !cfg.HasValue() {
		return nil
	}

	return &solace.SaslExternalConfig{}
}

func toSolaceFlow(cfg solacereceiver.FlowControl) solace.FlowControl {
	return solace.FlowControl{
		DelayedRetry: toFlowControlDelayedRetry(cfg.DelayedRetry),
	}
}

func toFlowControlDelayedRetry(cfg configoptional.Optional[solacereceiver.FlowControlDelayedRetry]) *solace.FlowControlDelayedRetry {
	if !cfg.HasValue() {
		return nil
	}
	return &solace.FlowControlDelayedRetry{
		Delay: cfg.Get().Delay,
	}
}
