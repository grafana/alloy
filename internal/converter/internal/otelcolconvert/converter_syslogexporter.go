package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/syslog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, syslogExporterConverter{})
}

type syslogExporterConverter struct{}

func (syslogExporterConverter) Factory() component.Factory {
	return syslogexporter.NewFactory()
}

func (syslogExporterConverter) InputComponentName() string {
	return "otelcol.exporter.syslog"
}

func (syslogExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toOtelcolExportersyslog(cfg.(*syslogexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "syslog"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolExportersyslog(cfg *syslogexporter.Config) *syslog.Arguments {
	return &syslog.Arguments{
		Queue:               toQueueArguments(cfg.QueueConfig),
		Retry:               toRetryArguments(cfg.RetryConfig),
		DebugMetrics:        common.DefaultValue[syslog.Arguments]().DebugMetrics,
		TLS:                 toTLSClientArguments(cfg.TLSSetting),
		Endpoint:            cfg.Endpoint,
		Port:                cfg.Port,
		Network:             cfg.Network,
		Protocol:            cfg.Protocol,
		Timeout:             cfg.Timeout,
		EnableOctetCounting: cfg.EnableOctetCounting,
	}
}
