// Package jaeger provides an otelcol.receiver.jaeger component.
package jaeger

import (
	"fmt"
	"maps"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfiggrpc "go.opentelemetry.io/collector/config/configgrpc"
	otelconfighttp "go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.jaeger",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := jaegerreceiver.NewFactory()
			return receiver.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.jaeger component.
type Arguments struct {
	Protocols ProtocolsArguments `alloy:"protocols,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
}

var _ receiver.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Protocols.GRPC == nil &&
		args.Protocols.ThriftHTTP == nil &&
		args.Protocols.ThriftBinary == nil &&
		args.Protocols.ThriftCompact == nil {

		return fmt.Errorf("at least one protocol must be enabled")
	}

	return nil
}

// Convert implements receiver.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	grpcProtocol, err := args.Protocols.GRPC.Convert()
	if err != nil {
		return nil, err
	}

	httpProtocol, err := args.Protocols.ThriftHTTP.Convert()
	if err != nil {
		return nil, err
	}
	return &jaegerreceiver.Config{
		Protocols: jaegerreceiver.Protocols{
			GRPC:             grpcProtocol,
			ThriftHTTP:       httpProtocol,
			ThriftBinaryUDP:  args.Protocols.ThriftBinary.Convert(),
			ThriftCompactUDP: args.Protocols.ThriftCompact.Convert(),
		},
	}, nil
}

// Extensions implements receiver.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	extensionMap := make(map[otelcomponent.ID]otelcomponent.Component)

	// Gets the extensions for the HTTP server and GRPC server
	if args.Protocols.ThriftHTTP != nil && args.Protocols.ThriftHTTP.HTTPServerArguments != nil {
		httpExtensions := args.Protocols.ThriftHTTP.HTTPServerArguments.Extensions()

		// Copies the extensions for the HTTP server into the map
		maps.Copy(extensionMap, httpExtensions)
	}

	if args.Protocols.GRPC != nil && args.Protocols.GRPC.GRPCServerArguments != nil {
		grpcExtensions := args.Protocols.GRPC.GRPCServerArguments.Extensions()

		// Copies the extensions for the GRPC server into the map.
		maps.Copy(extensionMap, grpcExtensions)
	}

	return extensionMap
}

// Exporters implements receiver.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements receiver.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ProtocolsArguments configures protocols for otelcol.receiver.jaeger to
// listen on.
type ProtocolsArguments struct {
	GRPC          *GRPC          `alloy:"grpc,block,optional"`
	ThriftHTTP    *ThriftHTTP    `alloy:"thrift_http,block,optional"`
	ThriftBinary  *ThriftBinary  `alloy:"thrift_binary,block,optional"`
	ThriftCompact *ThriftCompact `alloy:"thrift_compact,block,optional"`
}

type GRPC struct {
	GRPCServerArguments *otelcol.GRPCServerArguments `alloy:",squash"`
}

// SetToDefault implements syntax.Defaulter.
func (args *GRPC) SetToDefault() {
	*args = GRPC{
		GRPCServerArguments: &otelcol.GRPCServerArguments{
			Endpoint:  "0.0.0.0:14250",
			Transport: "tcp",
		},
	}
}

// Convert converts proto into the upstream type.
func (args *GRPC) Convert() (configoptional.Optional[otelconfiggrpc.ServerConfig], error) {
	if args == nil {
		return configoptional.None[otelconfiggrpc.ServerConfig](), nil
	}

	return args.GRPCServerArguments.Convert()
}

type ThriftHTTP struct {
	HTTPServerArguments *otelcol.HTTPServerArguments `alloy:",squash"`
}

// SetToDefault implements syntax.Defaulter.
func (args *ThriftHTTP) SetToDefault() {
	*args = ThriftHTTP{
		HTTPServerArguments: &otelcol.HTTPServerArguments{
			Endpoint:              "0.0.0.0:14268",
			CompressionAlgorithms: append([]string(nil), otelcol.DefaultCompressionAlgorithms...),
		},
	}
}

// Convert converts proto into the upstream type.
func (args *ThriftHTTP) Convert() (configoptional.Optional[otelconfighttp.ServerConfig], error) {
	if args == nil {
		return configoptional.None[otelconfighttp.ServerConfig](), nil
	}

	return args.HTTPServerArguments.Convert()
}

// ProtocolUDP configures a UDP server.
type ProtocolUDP struct {
	Endpoint         string           `alloy:"endpoint,attr,optional"`
	QueueSize        int              `alloy:"queue_size,attr,optional"`
	MaxPacketSize    units.Base2Bytes `alloy:"max_packet_size,attr,optional"`
	Workers          int              `alloy:"workers,attr,optional"`
	SocketBufferSize units.Base2Bytes `alloy:"socket_buffer_size,attr,optional"`
}

// Convert converts proto into the upstream type.
func (proto *ProtocolUDP) Convert() configoptional.Optional[jaegerreceiver.ProtocolUDP] {
	if proto == nil {
		return configoptional.None[jaegerreceiver.ProtocolUDP]()
	}

	return configoptional.Some(jaegerreceiver.ProtocolUDP{
		Endpoint: proto.Endpoint,
		ServerConfigUDP: jaegerreceiver.ServerConfigUDP{
			QueueSize:        proto.QueueSize,
			MaxPacketSize:    int(proto.MaxPacketSize),
			Workers:          proto.Workers,
			SocketBufferSize: int(proto.SocketBufferSize),
		},
	})
}

// ThriftCompact wraps ProtocolUDP and provides additional behavior.
type ThriftCompact struct {
	ProtocolUDP *ProtocolUDP `alloy:",squash"`
}

// SetToDefault implements syntax.Defaulter.
func (args *ThriftCompact) SetToDefault() {
	*args = ThriftCompact{
		ProtocolUDP: &ProtocolUDP{
			Endpoint:      "0.0.0.0:6831",
			QueueSize:     1_000,
			MaxPacketSize: 65 * units.KiB,
			Workers:       10,
		},
	}
}

// Convert converts proto into the upstream type.
func (args *ThriftCompact) Convert() configoptional.Optional[jaegerreceiver.ProtocolUDP] {
	if args == nil {
		return configoptional.None[jaegerreceiver.ProtocolUDP]()
	}

	return args.ProtocolUDP.Convert()
}

// ThriftCompact wraps ProtocolUDP and provides additional behavior.
type ThriftBinary struct {
	ProtocolUDP *ProtocolUDP `alloy:",squash"`
}

// SetToDefault implements syntax.Defaulter.
func (args *ThriftBinary) SetToDefault() {
	*args = ThriftBinary{
		ProtocolUDP: &ProtocolUDP{
			Endpoint:      "0.0.0.0:6832",
			QueueSize:     1_000,
			MaxPacketSize: 65 * units.KiB,
			Workers:       10,
		},
	}
}

// Convert converts proto into the upstream type.
func (args *ThriftBinary) Convert() configoptional.Optional[jaegerreceiver.ProtocolUDP] {
	if args == nil {
		return configoptional.None[jaegerreceiver.ProtocolUDP]()
	}

	return args.ProtocolUDP.Convert()
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
