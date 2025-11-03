package otelcolconvert

import (
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax/token/builder"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	"go.uber.org/zap/zapcore"
)

func convertTelemetry(file *builder.File, tel otelconftelemetry.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddAll(convertLogging(file, tel.Logs))
	diags.AddAll(convertMetrics(file, tel.Metrics))
	diags.AddAll(convertTraces(file, tel.Traces))

	return diags
}

func convertLoggingLevel(lvl zapcore.Level) logging.Level {
	switch lvl {
	case zapcore.DebugLevel:
		return logging.LevelDebug
	case zapcore.InfoLevel:
		return logging.LevelInfo
	case zapcore.WarnLevel:
		return logging.LevelWarn
	case zapcore.ErrorLevel:
		return logging.LevelError
	default:
		return logging.LevelDefault
	}
}

func convertLoggingFormat(encoding string) (logging.Format, diag.Diagnostics) {
	var diags diag.Diagnostics

	switch encoding {
	case "json":
		return logging.FormatJSON, diags
	case "console":
		// This is an info rather than a warning, because otherwise it'd spam too much.
		// TODO: Use a warning only when a user has explicitly set console logging?
		diags.Add(diag.SeverityLevelInfo, "console log format is not supported, using default log format")
		fallthrough
	default:
		return logging.FormatDefault, diags
	}
}

func convertLogging(file *builder.File, tel otelconftelemetry.LogsConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	format, formatDiags := convertLoggingFormat(tel.Encoding)
	diags.AddAll(formatDiags)

	logOpts := &logging.Options{
		Level:  convertLoggingLevel(tel.Level),
		Format: format,
	}

	block := common.NewBlockWithOverride([]string{"logging"}, "", logOpts)

	// Don't append an empty logging block.
	if len(block.Body().Nodes()) > 0 {
		file.Body().AppendBlock(block)
	}

	if tel.Development {
		diags.Add(diag.SeverityLevelWarn, "the service/telemetry/logs/development configuration is not supported")
	}
	if tel.DisableCaller {
		diags.Add(diag.SeverityLevelWarn, "the service/telemetry/logs/disable_caller configuration is not supported")
	}
	if tel.DisableStacktrace {
		diags.Add(diag.SeverityLevelWarn, "the service/telemetry/logs/disable_stacktrace configuration is not supported")
	}
	if tel.Sampling != nil {
		diags.Add(diag.SeverityLevelWarn, "the service/telemetry/logs/sampling configuration is not supported")
	}
	if len(tel.OutputPaths) > 0 &&
		// If this is set to the default (stderr) then it's the same as Alloy default.
		// There's no need to a diagnostic.
		!(len(tel.OutputPaths) == 1 && tel.OutputPaths[0] == "stderr") {

		diags.Add(diag.SeverityLevelCritical, "the service/telemetry/logs/output_paths configuration is not supported")
	}
	if tel.ErrorOutputPaths != nil &&
		// If this is set to the default (stderr) then it's the same as Alloy default.
		// There's no need to a diagnostic.
		!(len(tel.ErrorOutputPaths) == 1 && tel.ErrorOutputPaths[0] == "stderr") {

		diags.Add(diag.SeverityLevelCritical, "the service/telemetry/logs/error_output_paths configuration is not supported")
	}
	if tel.InitialFields != nil {
		diags.Add(diag.SeverityLevelCritical, "the service/telemetry/logs/initial_fields configuration is not supported")
	}
	if tel.Processors != nil {
		diags.Add(diag.SeverityLevelCritical, "the service/telemetry/logs/processors configuration is not supported")
	}

	return diags
}

// TODO: Support metrics conversion once upstream's "metrics" section is not experimental.
// We might also need a way to configure somethings in the config file instead of via cmd args.
// For example, the HTTTP server address.
func convertMetrics(_ *builder.File, tel otelconftelemetry.MetricsConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(tel.Readers) > 0 {
		diags.Add(diag.SeverityLevelWarn, "the service/telemetry/metrics/readers configuration is not supported - to gather Alloy's own telemetry refer to: https://grafana.com/docs/alloy/latest/collect/metamonitoring/")
	}

	return diags
}

// TODO: Support metrics conversion once upstream's "traces" section is not experimental.
func convertTraces(_ *builder.File, tel otelconftelemetry.TracesConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	if len(tel.Processors) > 0 {
		diags.Add(diag.SeverityLevelCritical, "the service/telemetry/traces/processors configuration is not supported")
	}

	if len(tel.Propagators) > 0 {
		diags.Add(diag.SeverityLevelCritical, "the service/telemetry/traces/propagators configuration is not supported")
	}

	return diags
}
