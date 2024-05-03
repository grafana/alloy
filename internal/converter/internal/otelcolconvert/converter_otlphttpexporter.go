package otelcolconvert

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/otlphttp"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/units"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
)

func init() {
	converters = append(converters, otlpHTTPExporterConverter{})
}

type otlpHTTPExporterConverter struct{}

func (otlpHTTPExporterConverter) Factory() component.Factory {
	return otlphttpexporter.NewFactory()
}

func (otlpHTTPExporterConverter) InputComponentName() string {
	return "otelcol.exporter.otlphttp"
}

func (otlpHTTPExporterConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val interface{}) interface{} {
		switch val.(type) {
		case auth.Handler:
			ext := state.LookupExtension(cfg.(*otlphttpexporter.Config).Auth.AuthenticatorID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return val
	}

	args := toOtelcolExporterOTLPHTTP(cfg.(*otlphttpexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "otlphttp"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolExporterOTLPHTTP(cfg *otlphttpexporter.Config) *otlphttp.Arguments {
	return &otlphttp.Arguments{
		Client:       otlphttp.HTTPClientArguments(toHTTPClientArguments(cfg.ClientConfig)),
		Queue:        toQueueArguments(cfg.QueueConfig),
		Retry:        toRetryArguments(cfg.RetryConfig),
		Encoding:     string(cfg.Encoding),
		DebugMetrics: common.DefaultValue[otlphttp.Arguments]().DebugMetrics,
	}
}

func toHTTPClientArguments(cfg confighttp.ClientConfig) otelcol.HTTPClientArguments {
	var a *auth.Handler
	if cfg.Auth != nil {
		a = &auth.Handler{}
	}

	var mic *int
	var ict *time.Duration
	defaults := confighttp.NewDefaultClientConfig()
	if mic = cfg.MaxIdleConns; mic == nil {
		mic = defaults.MaxIdleConns
	}
	if ict = cfg.IdleConnTimeout; ict == nil {
		ict = defaults.IdleConnTimeout
	}
	return otelcol.HTTPClientArguments{
		Endpoint:        cfg.Endpoint,
		Compression:     otelcol.CompressionType(cfg.Compression),
		TLS:             toTLSClientArguments(cfg.TLSSetting),
		ReadBufferSize:  units.Bytes(cfg.ReadBufferSize),
		WriteBufferSize: units.Bytes(cfg.WriteBufferSize),

		Timeout:              cfg.Timeout,
		Headers:              toHeadersMap(cfg.Headers),
		MaxIdleConns:         mic,
		MaxIdleConnsPerHost:  cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:      cfg.MaxConnsPerHost,
		IdleConnTimeout:      ict,
		DisableKeepAlives:    cfg.DisableKeepAlives,
		HTTP2PingTimeout:     cfg.HTTP2PingTimeout,
		HTTP2ReadIdleTimeout: cfg.HTTP2ReadIdleTimeout,

		Auth: a,
	}
}
