package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/filelog"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, filelogReceiverConverter{})
}

type filelogReceiverConverter struct{}

func (filelogReceiverConverter) Factory() component.Factory {
	return filelogreceiver.NewFactory()
}

func (filelogReceiverConverter) InputComponentName() string {
	return "otelcol.receiver.filelog"
}

func (filelogReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val any) any {
		switch val.(type) {
		case extension.ExtensionHandler:
			ext := state.LookupExtension(*cfg.(*filelogreceiver.FileLogConfig).StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toOtelcolReceiverfilelog(cfg.(*filelogreceiver.FileLogConfig))

	// TODO(@dehaansa) - find a way to convert the operators
	if len(cfg.(*filelogreceiver.FileLogConfig).Operators) > 0 {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("operators cannot currently be translated for %s", StringifyInstanceID(id)),
		)
	}

	// TODO(@dehaansa) - find a way to convert the metadata operators
	if cfg.(*filelogreceiver.FileLogConfig).InputConfig.Header != nil {
		diags.Add(
			diag.SeverityLevelWarn,
			fmt.Sprintf("header metadata_operators cannot currently be translated for %s", StringifyInstanceID(id)),
		)
	}

	block := common.NewBlockWithOverrideFn([]string{"otelcol", "receiver", "filelog"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toOtelcolReceiverfilelog(cfg *filelogreceiver.FileLogConfig) *filelog.Arguments {
	args := &filelog.Arguments{
		MatchCriteria:           *toOtelcolMatchCriteria(cfg.InputConfig.Criteria),
		PollInterval:            cfg.InputConfig.PollInterval,
		MaxConcurrentFiles:      cfg.InputConfig.MaxConcurrentFiles,
		MaxBatches:              cfg.InputConfig.MaxBatches,
		StartAt:                 cfg.InputConfig.StartAt,
		FingerprintSize:         units.Base2Bytes(cfg.InputConfig.FingerprintSize),
		MaxLogSize:              units.Base2Bytes(cfg.InputConfig.MaxLogSize),
		Encoding:                cfg.InputConfig.Encoding,
		FlushPeriod:             cfg.InputConfig.FlushPeriod,
		DeleteAfterRead:         cfg.InputConfig.DeleteAfterRead,
		IncludeFileRecordNumber: cfg.InputConfig.IncludeFileRecordNumber,
		Compression:             cfg.InputConfig.Compression,
		AcquireFSLock:           cfg.InputConfig.AcquireFSLock,
		MultilineConfig:         toOtelcolMultilineConfig(cfg.InputConfig.SplitConfig),
		TrimConfig:              toOtelcolTrimConfig(cfg.InputConfig.TrimConfig),
		Resolver:                filelog.Resolver(cfg.InputConfig.Resolver),
		DebugMetrics:            common.DefaultValue[filelog.Arguments]().DebugMetrics,
	}

	if cfg.StorageID != nil {
		args.Storage = &extension.ExtensionHandler{
			ID: *cfg.StorageID,
		}
	}

	if len(cfg.InputConfig.Attributes) > 0 {
		args.Attributes = make(map[string]string, len(cfg.InputConfig.Attributes))
		for k, v := range cfg.InputConfig.Attributes {
			args.Attributes[k] = string(v)
		}
	}

	if len(cfg.InputConfig.Resource) > 0 {
		args.Resource = make(map[string]string, len(cfg.InputConfig.Resource))
		for k, v := range cfg.InputConfig.Resource {
			args.Resource[k] = string(v)
		}
	}

	if cfg.InputConfig.Header != nil {
		args.Header = &filelog.HeaderConfig{
			Pattern: cfg.InputConfig.Header.Pattern,
		}
	}

	// This isn't done in a function because the type is not exported
	args.ConsumerRetry = otelcol.ConsumerRetryArguments{
		Enabled:         cfg.RetryOnFailure.Enabled,
		InitialInterval: cfg.RetryOnFailure.InitialInterval,
		MaxInterval:     cfg.RetryOnFailure.MaxInterval,
		MaxElapsedTime:  cfg.RetryOnFailure.MaxElapsedTime,
	}

	return args
}

func toOtelcolMatchCriteria(cfg matcher.Criteria) *filelog.MatchCriteria {
	return &filelog.MatchCriteria{
		Include:          cfg.Include,
		Exclude:          cfg.Exclude,
		ExcludeOlderThan: cfg.ExcludeOlderThan,
		OrderingCriteria: toOtelcolOrderingCriteria(cfg.OrderingCriteria),
	}
}

func toOtelcolOrderingCriteria(cfg matcher.OrderingCriteria) *filelog.OrderingCriteria {
	return &filelog.OrderingCriteria{
		Regex:   cfg.Regex,
		TopN:    cfg.TopN,
		SortBy:  toOtelcolSortBy(cfg.SortBy),
		GroupBy: cfg.GroupBy,
	}
}

func toOtelcolSortBy(cfg []matcher.Sort) []filelog.Sort {
	var sorts []filelog.Sort
	for _, s := range cfg {
		sorts = append(sorts,
			filelog.Sort{
				SortType:  s.SortType,
				RegexKey:  s.RegexKey,
				Ascending: s.Ascending,
				Layout:    s.Layout,
				Location:  s.Location,
			})
	}
	return sorts
}
