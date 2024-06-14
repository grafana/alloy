package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/debug"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/logging"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/debugexporter"
)

func init() {
	converters = append(converters, debugExporterConverter{})
}

type debugExporterConverter struct{}

func (debugExporterConverter) Factory() component.Factory {
	return debugexporter.NewFactory()
}

func (debugExporterConverter) InputComponentName() string {
	return "otelcol.exporter.debug"
}

func (debugExporterConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toDebugExporter(cfg.(*debugexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "debug"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toDebugExporter(cfg *debugexporter.Config) *debug.Arguments {
	return &debug.Arguments{
		Verbosity:          cfg.Verbosity.String(),
		SamplingInitial:    cfg.SamplingInitial,
		SamplingThereafter: cfg.SamplingThereafter,
		DebugMetrics:       common.DefaultValue[logging.Arguments]().DebugMetrics,
	}
}
