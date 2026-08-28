package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/redaction"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, redactionProcessorConverter{})
}

type redactionProcessorConverter struct{}

func (redactionProcessorConverter) Factory() component.Factory {
	return redactionprocessor.NewFactory()
}

func (redactionProcessorConverter) InputComponentName() string {
	return "otelcol.processor.redaction"
}

func (redactionProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toRedactionProcessor(state, id, cfg.(*redactionprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "redaction"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toRedactionProcessor(state *State, id componentstatus.InstanceID, cfg *redactionprocessor.Config) *redaction.Arguments {
	var (
		nextMetrics = state.Next(id, pipeline.SignalMetrics)
		nextLogs    = state.Next(id, pipeline.SignalLogs)
		nextTraces  = state.Next(id, pipeline.SignalTraces)
	)

	return &redaction.Arguments{
		AllowAllKeys:       cfg.AllowAllKeys,
		AllowedKeys:        cfg.AllowedKeys,
		IgnoredKeys:        cfg.IgnoredKeys,
		BlockedKeyPatterns: cfg.BlockedKeyPatterns,
		IgnoredKeyPatterns: cfg.IgnoredKeyPatterns,
		AllowedValues:      cfg.AllowedValues,
		BlockedValues:      cfg.BlockedValues,
		HashFunction:       string(cfg.HashFunction),
		HMACKey:            alloytypes.Secret(string(cfg.HMACKey)),
		RedactAllTypes:     cfg.RedactAllTypes,
		Summary:            cfg.Summary,
		URLSanitizer:       toURLSanitizer(cfg),
		DBSanitizer:        toDBSanitizer(cfg),

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
			Traces:  ToTokenizedConsumers(nextTraces),
		},

		DebugMetrics: common.DefaultValue[redaction.Arguments]().DebugMetrics,
	}
}

// toURLSanitizer reads the (internal-package-typed) URLSanitization field via
// its exported fields and returns a block only when it was configured.
func toURLSanitizer(cfg *redactionprocessor.Config) *redaction.URLSanitizerArguments {
	u := cfg.URLSanitization
	if !u.Enabled && len(u.Attributes) == 0 && u.SanitizeSpanName == nil {
		return nil
	}
	return &redaction.URLSanitizerArguments{
		Enabled:          u.Enabled,
		Attributes:       u.Attributes,
		SanitizeSpanName: u.SanitizeSpanName,
	}
}

// toDBSanitizer reads the (internal-package-typed) DBSanitizer field via its
// exported fields and returns a block only when it was configured.
func toDBSanitizer(cfg *redactionprocessor.Config) *redaction.DBSanitizerArguments {
	d := cfg.DBSanitizer

	dbs := []struct {
		enabled bool
		attrs   []string
	}{
		{d.SQLConfig.Enabled, d.SQLConfig.Attributes},
		{d.RedisConfig.Enabled, d.RedisConfig.Attributes},
		{d.ValkeyConfig.Enabled, d.ValkeyConfig.Attributes},
		{d.MemcachedConfig.Enabled, d.MemcachedConfig.Attributes},
		{d.MongoConfig.Enabled, d.MongoConfig.Attributes},
		{d.OpenSearchConfig.Enabled, d.OpenSearchConfig.Attributes},
		{d.ESConfig.Enabled, d.ESConfig.Attributes},
	}
	configured := d.SanitizeSpanName != nil
	for _, e := range dbs {
		if e.enabled || len(e.attrs) > 0 {
			configured = true
		}
	}
	if !configured {
		return nil
	}

	block := func(enabled bool, attrs []string) redaction.DBSanitizerBlock {
		return redaction.DBSanitizerBlock{Enabled: enabled, Attributes: attrs}
	}
	return &redaction.DBSanitizerArguments{
		SanitizeSpanName: d.SanitizeSpanName,
		SQL:              block(d.SQLConfig.Enabled, d.SQLConfig.Attributes),
		Redis:            block(d.RedisConfig.Enabled, d.RedisConfig.Attributes),
		Valkey:           block(d.ValkeyConfig.Enabled, d.ValkeyConfig.Attributes),
		Memcached:        block(d.MemcachedConfig.Enabled, d.MemcachedConfig.Attributes),
		Mongo:            block(d.MongoConfig.Enabled, d.MongoConfig.Attributes),
		OpenSearch:       block(d.OpenSearchConfig.Enabled, d.OpenSearchConfig.Attributes),
		ES:               block(d.ESConfig.Enabled, d.ESConfig.Attributes),
	}
}
