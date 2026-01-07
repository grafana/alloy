package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/file"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, fileExporterConverter{})
}

type fileExporterConverter struct{}

func (fileExporterConverter) Factory() component.Factory { return fileexporter.NewFactory() }

func (fileExporterConverter) InputComponentName() string {
	return "otelcol.exporter.file"
}

func (fileExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toFileExporter(diags, cfg.(*fileexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "file"}, label, args, common.GetAlloyTypesOverrideHook())

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toFileExporter(diags diag.Diagnostics, cfg *fileexporter.Config) *file.Arguments {
	args := &file.Arguments{
		Path:          cfg.Path,
		Format:        cfg.FormatType,
		Append:        cfg.Append,
		Compression:   cfg.Compression,
		FlushInterval: cfg.FlushInterval,
		DebugMetrics:  common.DefaultValue[file.Arguments]().DebugMetrics,
	}

	// Convert rotation settings if present
	if cfg.Rotation != nil {
		args.Rotation = &file.Rotation{
			MaxMegabytes: cfg.Rotation.MaxMegabytes,
			MaxDays:      cfg.Rotation.MaxDays,
			MaxBackups:   cfg.Rotation.MaxBackups,
			LocalTime:    cfg.Rotation.LocalTime,
		}
	}

	// Convert group_by settings if present
	if cfg.GroupBy != nil {
		args.GroupBy = &file.GroupBy{
			Enabled:           cfg.GroupBy.Enabled,
			ResourceAttribute: cfg.GroupBy.ResourceAttribute,
			MaxOpenFiles:      cfg.GroupBy.MaxOpenFiles,
		}
	}

	// Handle encoding if present (skip for now as it's not supported in our implementation)
	if cfg.Encoding != nil {
		// We don't support encoding yet, so we'll skip this
		// In a future implementation, this would be converted to a string representation
		diags.Add(
			diag.SeverityLevelWarn,
			"encoding parameter is not yet supported in the file exporter conversion and will be ignored",
		)
	}

	return args
}
