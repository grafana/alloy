package build

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/alecthomas/units"
	"github.com/mitchellh/mapstructure"

	"github.com/grafana/alloy/internal/component/loki/process/metric"
	"github.com/grafana/alloy/internal/component/loki/process/stages"
	"github.com/grafana/alloy/internal/converter/diag"
	promtailmetric "github.com/grafana/alloy/internal/loki/promtail/metric"
	promtailstages "github.com/grafana/alloy/internal/loki/promtail/stages"
	"github.com/grafana/alloy/internal/loki/util/flagext"
)

func convertStage(st any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	stage, ok := st.(promtailstages.PipelineStage)
	if !ok {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("invalid input YAML config, "+
			"make sure each stage of your pipeline is a YAML object (must end with a `:`), check stage `- %s`", st))
		return stages.StageConfig{}, false
	}
	if len(stage) != 1 {
		diags.Add(
			diag.SeverityLevelCritical,
			fmt.Sprintf("each pipeline stage must contain exactly one key, got: %v", stage),
		)
		return stages.StageConfig{}, false
	}

	for iName, iCfg := range stage {
		name, ok := iName.(string)
		if !ok {
			addInvalidStageError(diags, iCfg, fmt.Errorf("stage name must be a string, got %T", iName))
		}

		switch name {
		case promtailstages.StageTypeJSON:
			return convertJSONStage(iCfg, diags)
		case promtailstages.StageTypeLogfmt:
			return convertLogfmt(iCfg, diags)
		case promtailstages.StageTypeRegex:
			return convertRegex(iCfg, diags)
		case promtailstages.StageTypeReplace:
			return convertReplace(iCfg, diags)
		case promtailstages.StageTypeMetric:
			return convertMetrics(iCfg, diags)
		case promtailstages.StageTypeLabel:
			return convertLabels(iCfg, diags)
		case promtailstages.StageTypeLabelDrop:
			return convertLabelDrop(iCfg, diags)
		case promtailstages.StageTypeTimestamp:
			return convertTimestamp(iCfg, diags)
		case promtailstages.StageTypeOutput:
			return convertOutput(iCfg, diags)
		case promtailstages.StageTypeDocker:
			return convertDocker()
		case promtailstages.StageTypeCRI:
			return convertCRI(iCfg, diags)
		case promtailstages.StageTypeMatch:
			return convertMatch(iCfg, diags)
		case promtailstages.StageTypeTemplate:
			return convertTemplate(iCfg, diags)
		case promtailstages.StageTypeTenant:
			return convertTenant(iCfg, diags)
		case promtailstages.StageTypeDrop:
			return convertDrop(iCfg, diags)
		case promtailstages.StageTypeSampling:
			return convertSampling(iCfg, diags)
		case promtailstages.StageTypeLimit:
			return convertLimit(iCfg, diags)
		case promtailstages.StageTypeMultiline:
			return convertMultiline(iCfg, diags)
		case promtailstages.StageTypePack:
			return convertPack(iCfg, diags)
		case promtailstages.StageTypeLabelAllow:
			return convertLabelAllow(iCfg, diags)
		case promtailstages.StageTypeStaticLabels:
			return convertStaticLabels(iCfg, diags)
		case promtailstages.StageTypeDecolorize:
			return convertDecolorize(diags)
		case promtailstages.StageTypeEventLogMessage:
			return convertEventLogMessage(iCfg, diags)
		case promtailstages.StageTypeGeoIP:
			return convertGeoIP(iCfg, diags)
		case promtailstages.StageTypeStructuredMetadata:
			return convertStructuredMetadata(iCfg, diags)
		}
	}

	diags.Add(diag.SeverityLevelError, fmt.Sprintf("The converter does not support converting the provided pipeline stage: %v", st))
	return stages.StageConfig{}, false
}

func convertStructuredMetadata(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pLabels := &promtailstages.LabelsConfig{}
	if err := mapstructure.Decode(cfg, pLabels); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{StructuredMetadata: &stages.StructuredMetadataConfig{
		Values: *pLabels,
	}}, true
}

func convertGeoIP(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.GeoIPConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		GeoIPConfig: &stages.GeoIPConfig{
			DB:     pCfg.DB,
			Source: pCfg.Source,
			DBType: pCfg.DBType,
		},
	}, true
}

func convertEventLogMessage(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.EventLogMessageConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	result := &stages.EventLogMessageConfig{}
	result.SetToDefault()
	result.DropInvalidLabels = pCfg.DropInvalidLabels
	result.OverwriteExisting = pCfg.OverwriteExisting
	if pCfg.Source != nil {
		result.Source = *pCfg.Source
	}
	return stages.StageConfig{
		EventLogMessageConfig: result,
	}, true
}

func convertDecolorize(_ *diag.Diagnostics) (stages.StageConfig, bool) {
	return stages.StageConfig{DecolorizeConfig: &stages.DecolorizeConfig{}}, true
}

func convertStaticLabels(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.StaticLabelConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		StaticLabelsConfig: &stages.StaticLabelsConfig{
			Values: *pCfg,
		},
	}, true
}

func convertLabelAllow(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.LabelAllowConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		LabelAllowConfig: &stages.LabelAllowConfig{
			Values: *pCfg,
		},
	}, true
}

func convertPack(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pPack := &promtailstages.PackConfig{}
	// NOTE: using WeakDecode to match promtail behaviour
	if err := mapstructure.WeakDecode(cfg, pPack); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		PackConfig: &stages.PackConfig{
			Labels:          pPack.Labels,
			IngestTimestamp: defaultFalse(pPack.IngestTimestamp),
		},
	}, true
}

func convertMultiline(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pMulti := &promtailstages.MultilineConfig{}
	// NOTE: using WeakDecode to match promtail behaviour
	if err := mapstructure.WeakDecode(cfg, pMulti); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}

	var maxWaitTime time.Duration
	if pMulti.MaxWaitTime != nil {
		d, err := time.ParseDuration(*pMulti.MaxWaitTime)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("invalid pipeline_stages.multiline.max_wait_time: %s", *pMulti.MaxWaitTime))
		}
		maxWaitTime = d
	}

	return stages.StageConfig{
		MultilineConfig: &stages.MultilineConfig{
			Expression:   defaultEmpty(pMulti.Expression),
			MaxLines:     defaultZero(pMulti.MaxLines),
			MaxWaitTime:  maxWaitTime,
			TrimNewlines: true,
		},
	}, true
}

func convertLimit(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pLimit := &promtailstages.LimitConfig{}
	// NOTE: using WeakDecode to match promtail behaviour
	if err := mapstructure.WeakDecode(cfg, pLimit); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		LimitConfig: &stages.LimitConfig{
			Rate:              pLimit.Rate,
			Burst:             pLimit.Burst,
			Drop:              pLimit.Drop,
			ByLabelName:       pLimit.ByLabelName,
			MaxDistinctLabels: pLimit.MaxDistinctLabels,
		},
	}, true
}

func convertSampling(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pSampling := &promtailstages.SamplingConfig{}
	// NOTE: using WeakDecode to match promtail behaviour
	if err := mapstructure.WeakDecode(cfg, pSampling); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	fSampling := &stages.SamplingConfig{}
	fSampling.SetToDefault()
	fSampling.SamplingRate = pSampling.SamplingRate
	if pSampling.DropReason != nil {
		fSampling.DropReason = *pSampling.DropReason
	}
	return stages.StageConfig{
		SamplingConfig: fSampling,
	}, true
}

func convertDrop(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pDrop := &promtailstages.DropConfig{}
	// NOTE: using WeakDecode to match promtail behaviour
	if err := mapstructure.WeakDecode(cfg, pDrop); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}

	source := ""
	if pDrop.Source != nil {
		switch s := pDrop.Source.(type) {
		case []any:
			strs := make([]string, len(s))
			for i, v := range s {
				str, ok := v.(string)
				if !ok {
					diags.Add(
						diag.SeverityLevelError,
						fmt.Sprintf("invalid pipeline_stages.drop.source[%d] field type '%T': %v", i, v, v),
					)
					return stages.StageConfig{}, false
				}
				strs[i] = str
			}
			source = strings.Join(strs, ",")
		case string:
			source = s
		default:
			diags.Add(
				diag.SeverityLevelError,
				fmt.Sprintf("invalid pipeline_stages.drop.source field type '%T': %v", pDrop.Source, pDrop.Source),
			)
			return stages.StageConfig{}, false
		}
	}

	var olderThan time.Duration
	if pDrop.OlderThan != nil {
		d, err := time.ParseDuration(*pDrop.OlderThan)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("invalid pipeline_stages.drop.older_than field: %v", err))
			return stages.StageConfig{}, false
		}
		olderThan = d
	}

	var longerThan units.Base2Bytes
	if pDrop.LongerThan != nil {
		var pLongerThan flagext.ByteSize
		err := pLongerThan.Set(*pDrop.LongerThan)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("invalid pipeline_stages.drop.longer_than field: %v", err))
			return stages.StageConfig{}, false
		}
		longerThan, err = units.ParseBase2Bytes(fmt.Sprintf("%dB", pLongerThan.Val()))
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("invalid pipeline_stages.drop.longer_than field: failed Alloy type conversion: %v", err))
			return stages.StageConfig{}, false
		}
	}

	return stages.StageConfig{DropConfig: &stages.DropConfig{
		DropReason: defaultEmpty(pDrop.DropReason),
		Source:     source,
		Value:      defaultEmpty(pDrop.Value),
		Expression: defaultEmpty(pDrop.Expression),
		OlderThan:  olderThan,
		LongerThan: longerThan,
		Separator:  defaultEmpty(pDrop.Separator),
	}}, true
}

func convertTenant(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pTenant := &promtailstages.TenantConfig{}
	if err := mapstructure.Decode(cfg, pTenant); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{TenantConfig: &stages.TenantConfig{
		Label:  pTenant.Label,
		Source: pTenant.Source,
		Value:  pTenant.Value,
	}}, true
}

func convertTemplate(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pTemplate := &promtailstages.TemplateConfig{}
	if err := mapstructure.Decode(cfg, pTemplate); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{TemplateConfig: &stages.TemplateConfig{
		Source:   pTemplate.Source,
		Template: pTemplate.Template,
	}}, true
}

func convertMatch(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pMatch := &promtailstages.MatcherConfig{}
	if err := mapstructure.Decode(cfg, pMatch); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}

	// convert nested stages
	subStages := make([]stages.StageConfig, len(pMatch.Stages))
	for i, ps := range pMatch.Stages {
		if fs, ok := convertStage(ps, diags); ok {
			subStages[i] = fs
		}
	}

	return stages.StageConfig{MatchConfig: &stages.MatchConfig{
		Selector:     pMatch.Selector,
		Stages:       subStages,
		Action:       pMatch.Action,
		PipelineName: defaultEmpty(pMatch.PipelineName),
		DropReason:   defaultEmpty(pMatch.DropReason),
	}}, true
}

func convertCRI(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCRI := &promtailstages.CriConfig{}
	if err := mapstructure.Decode(cfg, pCRI); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}

	// Copied logic from Promtail: if MaxPartialLines is 0, default it to
	// MaxPartialLinesSize.
	if pCRI.MaxPartialLines == 0 {
		pCRI.MaxPartialLines = promtailstages.MaxPartialLinesSize
	}

	return stages.StageConfig{CRIConfig: &stages.CRIConfig{
		MaxPartialLines:            pCRI.MaxPartialLines,
		MaxPartialLineSize:         uint64(pCRI.MaxPartialLineSize),
		MaxPartialLineSizeTruncate: pCRI.MaxPartialLineSizeTruncate,
	}}, true
}

func convertDocker() (stages.StageConfig, bool) {
	return stages.StageConfig{DockerConfig: &stages.DockerConfig{}}, true
}

func convertOutput(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pOutput := &promtailstages.OutputConfig{}
	if err := mapstructure.Decode(cfg, pOutput); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{OutputConfig: &stages.OutputConfig{
		Source: pOutput.Source,
	}}, true
}

func convertTimestamp(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pTimestamp := &promtailstages.TimestampConfig{}
	if err := mapstructure.Decode(cfg, pTimestamp); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{TimestampConfig: &stages.TimestampConfig{
		Source:          pTimestamp.Source,
		Format:          pTimestamp.Format,
		FallbackFormats: pTimestamp.FallbackFormats,
		Location:        pTimestamp.Location,
		ActionOnFailure: defaultEmpty(pTimestamp.ActionOnFailure),
	},
	}, true
}

func convertLabelDrop(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pLabelDrop := &promtailstages.LabelDropConfig{}
	if err := mapstructure.Decode(cfg, pLabelDrop); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{LabelDropConfig: &stages.LabelDropConfig{
		Values: *pLabelDrop,
	}}, true
}

func convertLabels(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pLabels := &promtailstages.LabelsConfig{}
	if err := mapstructure.Decode(cfg, pLabels); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{LabelsConfig: &stages.LabelsConfig{
		Values: *pLabels,
	}}, true
}

func convertMetrics(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pMetrics := promtailstages.MetricsConfig{}
	if err := mapstructure.Decode(cfg, &pMetrics); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}

	var (
		sortedNames = make([]string, 0, len(pMetrics))
		fMetrics    = make([]stages.MetricConfig, 0, len(pMetrics))
	)
	// sort metric names to make conversion deterministic
	for name := range pMetrics {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)

	for _, name := range sortedNames {
		pMetric := pMetrics[name]
		fMetric, ok := toAlloyMetricsProcessStage(name, pMetric, diags)
		if !ok {
			return stages.StageConfig{}, false
		}
		fMetrics = append(fMetrics, fMetric)
	}

	return stages.StageConfig{MetricsConfig: &stages.MetricsConfig{
		Metrics: fMetrics,
	}}, true
}

func toAlloyMetricsProcessStage(name string, pMetric promtailstages.MetricConfig, diags *diag.Diagnostics) (stages.MetricConfig, bool) {
	var fMetric stages.MetricConfig

	var maxIdle time.Duration
	if pMetric.IdleDuration != nil {
		d, err := time.ParseDuration(*pMetric.IdleDuration)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("failed to parse duration: %s - %v", *pMetric.IdleDuration, err))
			return stages.MetricConfig{}, false
		}
		maxIdle = d
	}

	// Create metric according to type
	switch strings.ToLower(pMetric.MetricType) {
	case promtailstages.MetricTypeCounter:
		cfg, err := promtailmetric.ParseCounterConfig(pMetric.Config)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("failed to create counter metric process stage: %v", err))
			return stages.MetricConfig{}, false
		}
		fMetric.Counter = &metric.CounterConfig{
			Name:            name,
			Description:     pMetric.Description,
			Source:          defaultEmpty(pMetric.Source),
			Prefix:          pMetric.Prefix,
			MaxIdle:         maxIdle,
			Value:           defaultEmpty(cfg.Value),
			Action:          cfg.Action,
			MatchAll:        defaultFalse(cfg.MatchAll),
			CountEntryBytes: defaultFalse(cfg.CountBytes),
		}
	case promtailstages.MetricTypeGauge:
		cfg, err := promtailmetric.ParseGaugeConfig(pMetric.Config)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("failed to create gauge metric process stage: %v", err))
			return stages.MetricConfig{}, false
		}
		fMetric.Gauge = &metric.GaugeConfig{
			Name:        name,
			Description: pMetric.Description,
			Source:      defaultEmpty(pMetric.Source),
			Prefix:      pMetric.Prefix,
			MaxIdle:     maxIdle,
			Value:       defaultEmpty(cfg.Value),
			Action:      cfg.Action,
		}
	case promtailstages.MetricTypeHistogram:
		cfg, err := promtailmetric.ParseHistogramConfig(pMetric.Config)
		if err != nil {
			diags.Add(diag.SeverityLevelError, fmt.Sprintf("failed to create histogram metric process stage: %v", err))
			return stages.MetricConfig{}, false
		}
		fMetric.Histogram = &metric.HistogramConfig{
			Name:        name,
			Description: pMetric.Description,
			Source:      defaultEmpty(pMetric.Source),
			Prefix:      pMetric.Prefix,
			MaxIdle:     maxIdle,
			Value:       defaultEmpty(cfg.Value),
			Buckets:     cfg.Buckets,
		}
	}
	return fMetric, true
}

func convertReplace(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.ReplaceConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		ReplaceConfig: &stages.ReplaceConfig{
			Expression: pCfg.Expression,
			Source:     defaultEmpty(pCfg.Source),
			Replace:    pCfg.Replace,
		}}, true
}

func convertRegex(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.RegexConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		RegexConfig: &stages.RegexConfig{
			Expression: pCfg.Expression,
			Source:     pCfg.Source,
		}}, true
}

func convertLogfmt(cfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.LogfmtConfig{}
	if err := mapstructure.Decode(cfg, pCfg); err != nil {
		addInvalidStageError(diags, cfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		LogfmtConfig: &stages.LogfmtConfig{
			Source:  defaultEmpty(pCfg.Source),
			Mapping: pCfg.Mapping,
		}}, true
}

func convertJSONStage(iCfg any, diags *diag.Diagnostics) (stages.StageConfig, bool) {
	pCfg := &promtailstages.JSONConfig{}
	if err := mapstructure.Decode(iCfg, pCfg); err != nil {
		addInvalidStageError(diags, iCfg, err)
		return stages.StageConfig{}, false
	}
	return stages.StageConfig{
		JSONConfig: &stages.JSONConfig{
			Expressions:   pCfg.Expressions,
			Source:        pCfg.Source,
			DropMalformed: pCfg.DropMalformed,
		}}, true
}

func addInvalidStageError(diags *diag.Diagnostics, iCfg any, err error) {
	diags.Add(
		diag.SeverityLevelError,
		fmt.Sprintf("invalid pipeline stage config: %v - %v", iCfg, err),
	)
}

func defaultEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func defaultFalse(s *bool) bool {
	if s == nil {
		return false
	}
	return *s
}

func defaultZero(s *uint64) uint64 {
	if s == nil {
		return 0
	}
	return *s
}
