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
		nextTraces = state.Next(id, component.DataTypeTraces)
	)

	return &jaeger.Arguments{
		Protocols: jaeger.ProtocolsArguments{
			GRPC:          toJaegerGRPCArguments(cfg.GRPC),
			ThriftHTTP:    toJaegerThriftHTTPArguments(cfg.ThriftHTTP),
			ThriftBinary:  toJaegerThriftBinaryArguments(cfg.ThriftBinary),
			ThriftCompact: toJaegerThriftCompactArguments(cfg.ThriftCompact),
		},

		DebugMetrics: common.DefaultValue[jaeger.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Traces: ToTokenizedConsumers(nextTraces),
		},
	}
}

func toJaegerGRPCArguments(cfg *configgrpc.ServerConfig) *jaeger.GRPC {
	if cfg == nil {
		return nil
	}
	return &jaeger.GRPC{GRPCServerArguments: toGRPCServerArguments(cfg)}
}

func toJaegerThriftHTTPArguments(cfg *confighttp.ServerConfig) *jaeger.ThriftHTTP {
	if cfg == nil {
		return nil
	}
	return &jaeger.ThriftHTTP{HTTPServerArguments: toHTTPServerArguments(cfg)}
}

func toJaegerThriftBinaryArguments(cfg *jaegerreceiver.ProtocolUDP) *jaeger.ThriftBinary {
	if cfg == nil {
		return nil
	}
	return &jaeger.ThriftBinary{ProtocolUDP: toJaegerProtocolUDPArguments(cfg)}
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

func toJaegerThriftCompactArguments(cfg *jaegerreceiver.ProtocolUDP) *jaeger.ThriftCompact {
	if cfg == nil {
		return nil
	}
	return &jaeger.ThriftCompact{ProtocolUDP: toJaegerProtocolUDPArguments(cfg)}
}
