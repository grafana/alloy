package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/otlp"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
)

func init() {
	converters = append(converters, otlpExporterConverter{})
}

type otlpExporterConverter struct{}

func (otlpExporterConverter) Factory() component.Factory {
	return otlpexporter.NewFactory()
}

func (otlpExporterConverter) InputComponentName() string { return "otelcol.exporter.otlp" }

func (otlpExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val any) any {
		switch val.(type) {
		case auth.Handler:
			ext := state.LookupExtension(cfg.(*otlpexporter.Config).ClientConfig.Auth.Get().AuthenticatorID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		case extension.ExtensionHandler:
			queue := cfg.(*otlpexporter.Config).QueueConfig.GetOrInsertDefault()
			ext := state.LookupExtension(*queue.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return val
	}

	args := toOtelcolExporterOTLP(cfg.(*otlpexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "otlp"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolExporterOTLP(cfg *otlpexporter.Config) *otlp.Arguments {
	return &otlp.Arguments{
		Timeout: cfg.TimeoutConfig.Timeout,

		Queue: toQueueArguments(cfg.QueueConfig),
		Retry: toRetryArguments(cfg.RetryConfig),

		DebugMetrics: common.DefaultValue[otlp.Arguments]().DebugMetrics,

		Client: otlp.GRPCClientArguments(toGRPCClientArguments(cfg.ClientConfig)),
	}
}

func toQueueArguments(cfg configoptional.Optional[exporterhelper.QueueBatchConfig]) otelcol.QueueArguments {
	// Use GetOrInsertDefault() instead of Get(), because upstream OTel sets QueueConfig to a "default" flavor.
	// For a "default" flavour HasValue() returns false, even though it technically has a value.
	queueCfg := cfg.GetOrInsertDefault()

	if queueCfg == nil {
		return otelcol.QueueArguments{
			Enabled: false,
		}
	}

	sizer, err := queueCfg.Sizer.MarshalText()
	if err != nil {
		panic(fmt.Errorf("failed to marshal sizer: %w", err))
	}

	q := otelcol.QueueArguments{
		Enabled:         true, // Having a value in configoptional means the queue is enabled
		NumConsumers:    queueCfg.NumConsumers,
		QueueSize:       queueCfg.QueueSize,
		BlockOnOverflow: queueCfg.BlockOnOverflow,
		Sizer:           string(sizer),
	}

	if queueCfg.StorageID != nil {
		q.Storage = &extension.ExtensionHandler{
			ID: *queueCfg.StorageID,
		}
	}
	return q
}

func toRetryArguments(cfg configretry.BackOffConfig) otelcol.RetryArguments {
	return otelcol.RetryArguments{
		Enabled:             cfg.Enabled,
		InitialInterval:     cfg.InitialInterval,
		RandomizationFactor: cfg.RandomizationFactor,
		Multiplier:          cfg.Multiplier,
		MaxInterval:         cfg.MaxInterval,
		MaxElapsedTime:      cfg.MaxElapsedTime,
	}
}

func toGRPCClientArguments(cfg configgrpc.ClientConfig) otelcol.GRPCClientArguments {
	var a *auth.Handler
	if cfg.Auth.HasValue() {
		a = &auth.Handler{}
	}

	// Set default value for `balancer_name` to sync up with upstream's
	balancerName := cfg.BalancerName
	if balancerName == "" {
		balancerName = otelcol.DefaultBalancerName
	}

	return otelcol.GRPCClientArguments{
		Endpoint: cfg.Endpoint,

		Compression: otelcol.CompressionType(cfg.Compression),

		TLS:       toTLSClientArguments(cfg.TLS),
		Keepalive: toKeepaliveClientArguments(cfg.Keepalive.Get()),

		ReadBufferSize:  units.Base2Bytes(cfg.ReadBufferSize),
		WriteBufferSize: units.Base2Bytes(cfg.WriteBufferSize),
		WaitForReady:    cfg.WaitForReady,
		Headers:         toHeadersMap(cfg.Headers),
		BalancerName:    balancerName,
		Authority:       cfg.Authority,

		Authentication: a,
	}
}

func toTLSClientArguments(cfg configtls.ClientConfig) otelcol.TLSClientArguments {
	return otelcol.TLSClientArguments{
		TLSSetting: toTLSSetting(cfg.Config),

		Insecure:           cfg.Insecure,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
	}
}

func toKeepaliveClientArguments(cfg *configgrpc.KeepaliveClientConfig) *otelcol.KeepaliveClientArguments {
	if cfg == nil {
		return nil
	}

	return &otelcol.KeepaliveClientArguments{
		PingWait:            cfg.Time,
		PingResponseTimeout: cfg.Timeout,
		PingWithoutStream:   cfg.PermitWithoutStream,
	}
}

func toHeadersMap(cfg configopaque.MapList) map[string]string {
	res := make(map[string]string, len(cfg))
	cfg.Iter(func(k string, v configopaque.String) bool {
		res[k] = string(v)
		return true
	})
	return res
}
