package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/jaeger"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, jaegerReceiverConverter{})
}

type jaegerReceiverConverter struct{}

func (jaegerReceiverConverter) Factory() component.Factory { return jaegerreceiver.NewFactory() }

func (jaegerReceiverConverter) InputComponentName() string { return "" }

func (jaegerReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toJaegerReceiver(state, id, cfg.(*jaegerreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "jaeger"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toJaegerReceiver(state *State, id componentstatus.InstanceID, cfg *jaegerreceiver.Config) *jaeger.Arguments {
	var (
		nextTraces = state.Next(id, pipeline.SignalTraces)
	)

	return &jaeger.Arguments{
		Protocols: jaeger.ProtocolsArguments{
			GRPC:          toJaegerGRPCArguments(cfg.GRPC),
			ThriftHTTP:    toJaegerThriftHTTPArguments(cfg.ThriftHTTP),
			ThriftBinary:  toJaegerThriftBinaryArguments(cfg.ThriftBinaryUDP),
			ThriftCompact: toJaegerThriftCompactArguments(cfg.ThriftCompactUDP),
		},

		DebugMetrics: common.DefaultValue[jaeger.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Traces: ToTokenizedConsumers(nextTraces),
		},
	}
}

func toJaegerGRPCArguments(cfg configoptional.Optional[configgrpc.ServerConfig]) *jaeger.GRPC {
	if !cfg.HasValue() {
		return nil
	}
	return &jaeger.GRPC{GRPCServerArguments: toGRPCServerArguments(cfg.Get())}
}

func toJaegerThriftHTTPArguments(cfg configoptional.Optional[confighttp.ServerConfig]) *jaeger.ThriftHTTP {
	if !cfg.HasValue() {
		return nil
	}
	return &jaeger.ThriftHTTP{HTTPServerArguments: toHTTPServerArguments(cfg.Get())}
}

func toJaegerThriftBinaryArguments(cfg configoptional.Optional[jaegerreceiver.ProtocolUDP]) *jaeger.ThriftBinary {
	if !cfg.HasValue() {
		return nil
	}
	return &jaeger.ThriftBinary{ProtocolUDP: toJaegerProtocolUDPArguments(cfg.Get())}
}

func toJaegerProtocolUDPArguments(cfg *jaegerreceiver.ProtocolUDP) *jaeger.ProtocolUDP {
	if cfg == nil {
		return nil
	}

	return &jaeger.ProtocolUDP{
		Endpoint:         cfg.Endpoint,
		QueueSize:        cfg.QueueSize,
		MaxPacketSize:    units.Base2Bytes(cfg.MaxPacketSize),
		Workers:          cfg.Workers,
		SocketBufferSize: units.Base2Bytes(cfg.SocketBufferSize),
	}
}

func toJaegerThriftCompactArguments(cfg configoptional.Optional[jaegerreceiver.ProtocolUDP]) *jaeger.ThriftCompact {
	if !cfg.HasValue() {
		return nil
	}
	return &jaeger.ThriftCompact{ProtocolUDP: toJaegerProtocolUDPArguments(cfg.Get())}
}
