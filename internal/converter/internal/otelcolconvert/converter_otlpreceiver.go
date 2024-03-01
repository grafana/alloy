package otelcolconvert

import (
	"fmt"

	"github.com/alecthomas/units"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver/otlp"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/river/rivertypes"
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

func (otlpReceiverConverter) ConvertAndAppend(state *state, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.FlowComponentLabel()

	args := toOtelcolReceiverOTLP(state, id, cfg.(*otlpreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "otlp"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", stringifyInstanceID(id), stringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceiverOTLP(state *state, id component.InstanceID, cfg *otlpreceiver.Config) *otlp.Arguments {
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
			Metrics: toTokenizedConsumers(nextMetrics),
			Logs:    toTokenizedConsumers(nextLogs),
			Traces:  toTokenizedConsumers(nextTraces),
		},
	}
}

func toGRPCServerArguments(cfg *configgrpc.GRPCServerSettings) *otelcol.GRPCServerArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.GRPCServerArguments{
		Endpoint:  cfg.NetAddr.Endpoint,
		Transport: cfg.NetAddr.Transport,

		TLS: toTLSServerArguments(cfg.TLSSetting),

		MaxRecvMsgSize:       units.Base2Bytes(cfg.MaxRecvMsgSizeMiB) * units.MiB,
		MaxConcurrentStreams: cfg.MaxConcurrentStreams,
		ReadBufferSize:       units.Base2Bytes(cfg.ReadBufferSize),
		WriteBufferSize:      units.Base2Bytes(cfg.WriteBufferSize),

		Keepalive: toKeepaliveServerArguments(cfg.Keepalive),

		IncludeMetadata: cfg.IncludeMetadata,
	}
}

func toTLSServerArguments(cfg *configtls.TLSServerSetting) *otelcol.TLSServerArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.TLSServerArguments{
		TLSSetting: toTLSSetting(cfg.TLSSetting),

		ClientCAFile: cfg.ClientCAFile,
	}
}

func toTLSSetting(cfg configtls.TLSSetting) otelcol.TLSSetting {
	return otelcol.TLSSetting{
		CA:             string(cfg.CAPem),
		CAFile:         cfg.CAFile,
		Cert:           string(cfg.CertPem),
		CertFile:       cfg.CertFile,
		Key:            rivertypes.Secret(cfg.KeyPem),
		KeyFile:        cfg.KeyFile,
		MinVersion:     cfg.MinVersion,
		MaxVersion:     cfg.MaxVersion,
		ReloadInterval: cfg.ReloadInterval,
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
		HTTPServerArguments: toHTTPServerArguments(cfg.HTTPServerSettings),

		TracesURLPath:  cfg.TracesURLPath,
		MetricsURLPath: cfg.MetricsURLPath,
		LogsURLPath:    cfg.LogsURLPath,
	}
}

func toHTTPServerArguments(cfg *confighttp.HTTPServerSettings) *otelcol.HTTPServerArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.HTTPServerArguments{
		Endpoint: cfg.Endpoint,

		TLS: toTLSServerArguments(cfg.TLSSetting),

		CORS: toCORSArguments(cfg.CORS),

		MaxRequestBodySize: units.Base2Bytes(cfg.MaxRequestBodySize),
		IncludeMetadata:    cfg.IncludeMetadata,
	}
}

func toCORSArguments(cfg *confighttp.CORSSettings) *otelcol.CORSArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.CORSArguments{
		AllowedOrigins: cfg.AllowedOrigins,
		AllowedHeaders: cfg.AllowedHeaders,

		MaxAge: cfg.MaxAge,
	}
}
