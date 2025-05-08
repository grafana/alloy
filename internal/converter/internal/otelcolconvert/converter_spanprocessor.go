package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/span"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, spanProcessorConverter{})
}

type spanProcessorConverter struct{}

func (spanProcessorConverter) Factory() component.Factory { return spanprocessor.NewFactory() }

func (spanProcessorConverter) InputComponentName() string { return "otelcol.processor.span" }

func (spanProcessorConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toSpanProcessor(state, id, cfg.(*spanprocessor.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "processor", "span"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toSpanProcessor(state *State, id componentstatus.InstanceID, cfg *spanprocessor.Config) *span.Arguments {
	var (
		nextTraces = state.Next(id, pipeline.SignalTraces)
	)

	var setStatus *span.Status
	if cfg.SetStatus != nil {
		setStatus = &span.Status{
			Code:        cfg.SetStatus.Code,
			Description: cfg.SetStatus.Description,
		}
	}

	var toAttributes *span.ToAttributes
	if cfg.Rename.ToAttributes != nil {
		toAttributes = &span.ToAttributes{
			Rules:            cfg.Rename.ToAttributes.Rules,
			BreakAfterMatch:  cfg.Rename.ToAttributes.BreakAfterMatch,
			KeepOriginalName: cfg.Rename.ToAttributes.KeepOriginalName,
		}
	}

	return &span.Arguments{
		Match: otelcol.MatchConfig{
			Include: toMatchProperties(encodeMapstruct(cfg.Include)),
			Exclude: toMatchProperties(encodeMapstruct(cfg.Exclude)),
		},
		Name: span.Name{
			FromAttributes: cfg.Rename.FromAttributes,
			Separator:      cfg.Rename.Separator,
			ToAttributes:   toAttributes,
		},
		SetStatus: setStatus,
		Output: &otelcol.ConsumerArguments{
			Traces: ToTokenizedConsumers(nextTraces),
		},
		DebugMetrics: common.DefaultValue[span.Arguments]().DebugMetrics,
	}
}

func toMatchProperties(cfg map[string]any) *otelcol.MatchProperties {
	if cfg == nil {
		return nil
	}

	return &otelcol.MatchProperties{
		MatchType:        encodeString(cfg["match_type"]),
		RegexpConfig:     toRegexpConfig(cfg),
		LogSeverity:      toLogSeverity(cfg),
		Services:         cfg["services"].([]string),
		SpanNames:        cfg["span_names"].([]string),
		LogBodies:        cfg["log_bodies"].([]string),
		LogSeverityTexts: cfg["log_severity_texts"].([]string),
		MetricNames:      cfg["metric_names"].([]string),
		SpanKinds:        cfg["span_kinds"].([]string),
		Attributes:       toOtelcolAttributes(encodeMapslice(cfg["attributes"])),
		Resources:        toOtelcolAttributes(encodeMapslice(cfg["resources"])),
		Libraries:        toOtelcolInstrumentationLibrary(encodeMapslice(cfg["libraries"])),
	}
}

func toOtelcolAttributes(in []map[string]any) []otelcol.Attribute {
	res := make([]otelcol.Attribute, 0, len(in))

	for _, a := range in {
		res = append(res, otelcol.Attribute{
			Key:   a["key"].(string),
			Value: a["value"],
		})
	}

	return res
}

func toOtelcolInstrumentationLibrary(in []map[string]any) []otelcol.InstrumentationLibrary {
	res := make([]otelcol.InstrumentationLibrary, 0, len(in))

	for _, l := range in {
		res = append(res, otelcol.InstrumentationLibrary{
			Name:    l["name"].(string),
			Version: l["version"].(*string),
		})
	}
	return res
}

func toRegexpConfig(cfg map[string]any) *otelcol.RegexpConfig {
	if cfg["regexp_config"] == nil {
		return nil
	}

	rc := cfg["regexp_config"].(map[string]any)

	return &otelcol.RegexpConfig{
		CacheEnabled:       rc["cache_enabled"].(bool),
		CacheMaxNumEntries: rc["cache_max_num_entries"].(int),
	}
}
func toLogSeverity(cfg map[string]any) *otelcol.LogSeverityNumberMatchProperties {
	if cfg["log_severity_number"] == nil {
		return nil
	}

	// Theres's a nested type, so we have to re-encode the field.
	ls := encodeMapstruct(cfg["log_severity_number"])
	if ls == nil {
		return nil
	}

	// This should never error out, but there's no 'unknown' severity level to
	// return in case it did.
	sn, err := otelcol.LookupSeverityNumber(ls["min"].(plog.SeverityNumber))
	if err != nil {
		panic(err)
	}

	return &otelcol.LogSeverityNumberMatchProperties{
		Min:            sn,
		MatchUndefined: ls["match_undefined"].(bool),
	}
}
