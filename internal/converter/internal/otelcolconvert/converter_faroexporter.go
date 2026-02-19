package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/faro"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/confighttp"
)

func init() {
	converters = append(converters, faroExporterConverter{})
}

type faroExporterConverter struct{}

func (faroExporterConverter) Factory() component.Factory {
	return faroexporter.NewFactory()
}

func (faroExporterConverter) InputComponentName() string {
	return "otelcol.exporter.faro"
}

func (faroExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val any) any {
		switch val.(type) {
		case auth.Handler:
			ext := state.LookupExtension(cfg.(*faroexporter.Config).ClientConfig.Auth.Get().AuthenticatorID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		case extension.ExtensionHandler:
			queue := cfg.(*faroexporter.Config).QueueConfig.GetOrInsertDefault()
			ext := state.LookupExtension(*queue.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toFaroExporter(cfg.(*faroexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "faro"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toFaroExporter(cfg *faroexporter.Config) *faro.Arguments {
	return &faro.Arguments{
		Client:       toFaroHTTPClientArguments(cfg.ClientConfig),
		Queue:        toQueueArguments(cfg.QueueConfig),
		Retry:        toRetryArguments(cfg.RetryConfig),
		DebugMetrics: common.DefaultValue[faro.Arguments]().DebugMetrics,
	}
}

func toFaroHTTPClientArguments(cfg confighttp.ClientConfig) faro.HTTPClientArguments {
	var a *auth.Handler
	if cfg.Auth.HasValue() {
		a = &auth.Handler{}
	}

	return faro.HTTPClientArguments{
		Endpoint:        cfg.Endpoint,
		ProxyUrl:        cfg.ProxyURL,
		Compression:     otelcol.CompressionType(cfg.Compression),
		TLS:             toTLSClientArguments(cfg.TLS),
		ReadBufferSize:  units.Base2Bytes(cfg.ReadBufferSize),
		WriteBufferSize: units.Base2Bytes(cfg.WriteBufferSize),

		Timeout:              cfg.Timeout,
		Headers:              toHeadersMap(cfg.Headers),
		MaxIdleConns:         cfg.MaxIdleConns,
		MaxIdleConnsPerHost:  cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:      cfg.MaxConnsPerHost,
		IdleConnTimeout:      cfg.IdleConnTimeout,
		DisableKeepAlives:    cfg.DisableKeepAlives,
		HTTP2PingTimeout:     cfg.HTTP2PingTimeout,
		HTTP2ReadIdleTimeout: cfg.HTTP2ReadIdleTimeout,
		ForceAttemptHTTP2:    cfg.ForceAttemptHTTP2,

		Authentication: a,
	}
}
