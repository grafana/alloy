package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/syslog"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
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
	overrideHook := func(val any) any {
		switch val.(type) {
		case extension.ExtensionHandler:
			queue := cfg.(*syslogexporter.Config).QueueSettings.GetOrInsertDefault()
			ext := state.LookupExtension(*queue.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toOtelcolExportersyslog(cfg.(*syslogexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "syslog"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolExportersyslog(cfg *syslogexporter.Config) *syslog.Arguments {
	return &syslog.Arguments{
		Queue:               toQueueArguments(cfg.QueueSettings),
		Retry:               toRetryArguments(cfg.BackOffConfig),
		DebugMetrics:        common.DefaultValue[syslog.Arguments]().DebugMetrics,
		TLS:                 toTLSClientArguments(cfg.TLS),
		Endpoint:            cfg.Endpoint,
		Port:                cfg.Port,
		Network:             cfg.Network,
		Protocol:            config.SysLogFormat(cfg.Protocol),
		Timeout:             cfg.TimeoutSettings.Timeout,
		EnableOctetCounting: cfg.EnableOctetCounting,
	}
}
