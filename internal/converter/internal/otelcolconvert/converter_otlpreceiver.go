package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/otlp"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
)

func init() {
	converters = append(converters, otlpReceiverConverter{})
}

type otlpReceiverConverter struct{}

func (otlpReceiverConverter) Factory() component.Factory { return otlpreceiver.NewFactory() }

func (otlpReceiverConverter) InputComponentName() string { return "" }

func (otlpReceiverConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolReceiverOTLP(state, id, cfg.(*otlpreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "otlp"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceiverOTLP(state *State, id component.InstanceID, cfg *otlpreceiver.Config) *otlp.Arguments {
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
		nextLogs    = state.Next(id, component.DataTypeLogs)
		nextTraces  = state.Next(id, component.DataTypeTraces)
	)

	return &otlp.Arguments{
		GRPC: (*otlp.GRPCServerArguments)(toGRPCServerArguments(cfg.GRPC)),
		HTTP: toHTTPConfigArguments(cfg.HTTP),

		DebugMetrics: common.DefaultValue[otlp.Arguments]().DebugMetrics,

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},
	}
}

func toGRPCServerArguments(cfg *configgrpc.ServerConfig) *otelcol.GRPCServerArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.GRPCServerArguments{
		Endpoint:  cfg.NetAddr.Endpoint,
		Transport: string(cfg.NetAddr.Transport),

		TLS: toTLSServerArguments(cfg.TLSSetting),

		MaxRecvMsgSize:       units.Base2Bytes(cfg.MaxRecvMsgSizeMiB) * units.MiB,
		MaxConcurrentStreams: cfg.MaxConcurrentStreams,
		ReadBufferSize:       units.Base2Bytes(cfg.ReadBufferSize),
		WriteBufferSize:      units.Base2Bytes(cfg.WriteBufferSize),

		Keepalive: toKeepaliveServerArguments(cfg.Keepalive),

		IncludeMetadata: cfg.IncludeMetadata,
	}
}

func toTLSServerArguments(cfg *configtls.ServerConfig) *otelcol.TLSServerArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.TLSServerArguments{
		TLSSetting: toTLSSetting(cfg.Config),

		ClientCAFile: cfg.ClientCAFile,
	}
}

func toTLSSetting(cfg configtls.Config) otelcol.TLSSetting {
	return otelcol.TLSSetting{
		CA:                       string(cfg.CAPem),
		CAFile:                   cfg.CAFile,
		Cert:                     string(cfg.CertPem),
		CertFile:                 cfg.CertFile,
		Key:                      alloytypes.Secret(cfg.KeyPem),
		KeyFile:                  cfg.KeyFile,
		MinVersion:               cfg.MinVersion,
		MaxVersion:               cfg.MaxVersion,
		ReloadInterval:           cfg.ReloadInterval,
		IncludeSystemCACertsPool: cfg.IncludeSystemCACertsPool,
		//TODO(ptodev): Do we need to copy this slice?
		CipherSuites: cfg.CipherSuites,
	}
}

func toKeepaliveServerArguments(cfg *configgrpc.KeepaliveServerConfig) *otelcol.KeepaliveServerArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.KeepaliveServerArguments{
		ServerParameters:  toKeepaliveServerParameters(cfg.ServerParameters),
		EnforcementPolicy: toKeepaliveEnforcementPolicy(cfg.EnforcementPolicy),
	}
}

func toKeepaliveServerParameters(cfg *configgrpc.KeepaliveServerParameters) *otelcol.KeepaliveServerParamaters {
	if cfg == nil {
		return nil
	}

	return &otelcol.KeepaliveServerParamaters{
		MaxConnectionIdle:     cfg.MaxConnectionIdle,
		MaxConnectionAge:      cfg.MaxConnectionAge,
		MaxConnectionAgeGrace: cfg.MaxConnectionAgeGrace,
		Time:                  cfg.Time,
		Timeout:               cfg.Timeout,
	}
}

func toKeepaliveEnforcementPolicy(cfg *configgrpc.KeepaliveEnforcementPolicy) *otelcol.KeepaliveEnforcementPolicy {
	if cfg == nil {
		return nil
	}

	return &otelcol.KeepaliveEnforcementPolicy{
		MinTime:             cfg.MinTime,
		PermitWithoutStream: cfg.PermitWithoutStream,
	}
}

func toHTTPConfigArguments(cfg *otlpreceiver.HTTPConfig) *otlp.HTTPConfigArguments {
	if cfg == nil {
		return nil
	}

	return &otlp.HTTPConfigArguments{
		HTTPServerArguments: toHTTPServerArguments(cfg.ServerConfig),

		TracesURLPath:  cfg.TracesURLPath,
		MetricsURLPath: cfg.MetricsURLPath,
		LogsURLPath:    cfg.LogsURLPath,
	}
}

func toHTTPServerArguments(cfg *confighttp.ServerConfig) *otelcol.HTTPServerArguments {
	if cfg == nil {
		return nil
	}

	var compressionAlgorithms []string
	if len(cfg.CompressionAlgorithms) > 0 {
		compressionAlgorithms = append([]string{}, cfg.CompressionAlgorithms...)
	} else {
		compressionAlgorithms = append([]string{}, otelcol.DefaultCompressionAlgorithms...)
	}

	return &otelcol.HTTPServerArguments{
		Endpoint: cfg.Endpoint,

		TLS: toTLSServerArguments(cfg.TLSSetting),

		CORS: toCORSArguments(cfg.CORS),

		MaxRequestBodySize: units.Base2Bytes(cfg.MaxRequestBodySize),
		IncludeMetadata:    cfg.IncludeMetadata,

		CompressionAlgorithms: compressionAlgorithms,
	}
}

func toCORSArguments(cfg *confighttp.CORSConfig) *otelcol.CORSArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.CORSArguments{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedHeaders: cfg.AllowedHeaders,

		MaxAge: cfg.MaxAge,
	}
}
