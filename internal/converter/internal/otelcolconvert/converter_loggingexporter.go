package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/logging"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.uber.org/zap/zapcore"
)

func init() {
	converters = append(converters, loggingExporterConverter{})
}

type loggingExporterConverter struct{}

func (loggingExporterConverter) Factory() component.Factory {
	return loggingexporter.NewFactory()
}

func (loggingExporterConverter) InputComponentName() string {
	return "otelcol.exporter.logging"
}

func (loggingExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	args := toOtelcolExporterLogging(cfg.(*loggingexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "logging"}, label, args, nil)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	diags.AddAll(common.ValidateSupported(common.NotEquals,
		cfg.(*loggingexporter.Config).LogLevel,
		zapcore.InfoLevel,
		"otelcol logging exporter loglevel",
		"use verbosity instead since loglevel is deprecated"))

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolExporterLogging(cfg *loggingexporter.Config) *logging.Arguments {
	return &logging.Arguments{
		Verbosity:          cfg.Verbosity,
		SamplingInitial:    cfg.SamplingInitial,
		SamplingThereafter: cfg.SamplingThereafter,
		DebugMetrics:       common.DefaultValue[logging.Arguments]().DebugMetrics,
	}
}
