package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/storage/file"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, fileStorageExtensionConverter{})
}

type fileStorageExtensionConverter struct{}

func (fileStorageExtensionConverter) Factory() component.Factory {
	return filestorage.NewFactory()
}

func (fileStorageExtensionConverter) InputComponentName() string { return "otelcol.storage.file" }

func (fileStorageExtensionConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toFileStorageExtension(cfg.(*filestorage.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "storage", "file"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toFileStorageExtension(cfg *filestorage.Config) *file.Arguments {
	return &file.Arguments{
		Directory: cfg.Directory,
		Timeout:   cfg.Timeout,
		Compaction: &file.CompactionConfig{
			OnStart:                    cfg.Compaction.OnStart,
			OnRebound:                  cfg.Compaction.OnRebound,
			Directory:                  cfg.Compaction.Directory,
			ReboundNeededThresholdMiB:  cfg.Compaction.ReboundNeededThresholdMiB,
			ReboundTriggerThresholdMiB: cfg.Compaction.ReboundTriggerThresholdMiB,
			MaxTransactionSize:         cfg.Compaction.MaxTransactionSize,
			CheckInterval:              cfg.Compaction.CheckInterval,
			CleanupOnStart:             cfg.Compaction.CleanupOnStart,
		},
		FSync:                cfg.FSync,
		CreateDirectory:      cfg.CreateDirectory,
		DirectoryPermissions: cfg.DirectoryPermissions,

		DebugMetrics: common.DefaultValue[file.Arguments]().DebugMetrics,
	}
}
