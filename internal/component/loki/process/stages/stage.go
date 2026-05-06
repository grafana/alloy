package stages

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

// Processor takes an existing set of labels, timestamp and log entry and returns either a possibly mutated
// timestamp and log entry
type Processor interface {
	Process(labels model.LabelSet, extracted map[string]any, time *time.Time, entry *string)
}

type Entry struct {
	Extracted map[string]any
	loki.Entry
}

// Stage can receive entries via an inbound channel and forward mutated entries to an outbound channel.
type Stage interface {
	Run(chan Entry) chan Entry
	Cleanup()
}

// stageProcessor Allow to transform a Processor (old synchronous pipeline stage) into an async Stage
type stageProcessor struct {
	Processor
}

func (s stageProcessor) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		s.Process(e.Labels, e.Extracted, &e.Timestamp, &e.Line)
		return e
	})
}

func toStage(p Processor) Stage {
	return &stageProcessor{
		Processor: p,
	}
}

// New creates a new stage for the given type and configuration.
func New(slogger *slog.Logger, cfg StageConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	var (
		s   Stage
		err error
	)
	switch {
	case cfg.DockerConfig != nil:
		s, err = NewDocker(slogger, registerer, minStability)
		if err != nil {
			return nil, err
		}
	case cfg.CRIConfig != nil:
		s, err = NewCRI(slogger, *cfg.CRIConfig, registerer, minStability)
		if err != nil {
			return nil, err
		}
	case cfg.JSONConfig != nil:
		s, err = newJSONStage(slogger, *cfg.JSONConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LogfmtConfig != nil:
		s, err = newLogfmtStage(slogger, *cfg.LogfmtConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LuhnFilterConfig != nil:
		s, err = newLuhnFilterStage(*cfg.LuhnFilterConfig)
		if err != nil {
			return nil, err
		}
	case cfg.MetricsConfig != nil:
		s, err = newMetricStage(slogger, *cfg.MetricsConfig, registerer)
		if err != nil {
			return nil, err
		}
	case cfg.LabelsConfig != nil:
		s, err = newLabelStage(slogger, *cfg.LabelsConfig)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadata != nil:
		s, err = newStructuredMetadataStage(slogger, *cfg.StructuredMetadata)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadataDropConfig != nil:
		s, err = newStructuredMetadataDropStage(slogger, *cfg.StructuredMetadataDropConfig)
		if err != nil {
			return nil, err
		}
	case cfg.RegexConfig != nil:
		s, err = newRegexStage(slogger, *cfg.RegexConfig)
		if err != nil {
			return nil, err
		}
	case cfg.TimestampConfig != nil:
		s, err = newTimestampStage(slogger, *cfg.TimestampConfig)
		if err != nil {
			return nil, err
		}
	case cfg.OutputConfig != nil:
		s, err = newOutputStage(slogger, *cfg.OutputConfig)
		if err != nil {
			return nil, err
		}
	case cfg.MatchConfig != nil:
		s, err = newMatcherStage(slogger, *cfg.MatchConfig, registerer, minStability)
		if err != nil {
			return nil, err
		}
	case cfg.TemplateConfig != nil:
		s, err = newTemplateStage(slogger, *cfg.TemplateConfig)
		if err != nil {
			return nil, err
		}
	case cfg.TenantConfig != nil:
		s, err = newTenantStage(slogger, *cfg.TenantConfig)
		if err != nil {
			return nil, err
		}
	case cfg.ReplaceConfig != nil:
		s, err = newReplaceStage(slogger, *cfg.ReplaceConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LimitConfig != nil:
		s, err = newLimitStage(slogger, *cfg.LimitConfig, registerer)
		if err != nil {
			return nil, err
		}
	case cfg.DropConfig != nil:
		s, err = newDropStage(slogger, *cfg.DropConfig, registerer)
		if err != nil {
			return nil, err
		}
	case cfg.MultilineConfig != nil:
		s, err = newMultilineStage(slogger, *cfg.MultilineConfig)
		if err != nil {
			return nil, err
		}
	case cfg.PackConfig != nil:
		s = newPackStage(slogger, *cfg.PackConfig, registerer)
	case cfg.LabelAllowConfig != nil:
		s, err = newLabelAllowStage(*cfg.LabelAllowConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LabelDropConfig != nil:
		s, err = newLabelDropStage(*cfg.LabelDropConfig)
		if err != nil {
			return nil, err
		}
	case cfg.StaticLabelsConfig != nil:
		s, err = newStaticLabelsStage(*cfg.StaticLabelsConfig)
		if err != nil {
			return nil, err
		}
	case cfg.GeoIPConfig != nil:
		s, err = newGeoIPStage(slogger, *cfg.GeoIPConfig)
		if err != nil {
			return nil, err
		}
	case cfg.DecolorizeConfig != nil:
		s, err = newDecolorizeStage(*cfg.DecolorizeConfig)
		if err != nil {
			return nil, err
		}
	case cfg.SamplingConfig != nil:
		s = newSamplingStage(slogger, *cfg.SamplingConfig, registerer)
	case cfg.EventLogMessageConfig != nil:
		s = newEventLogMessageStage(slogger, cfg.EventLogMessageConfig)
	case cfg.WindowsEventConfig != nil:
		s = newWindowsEventStage(slogger, cfg.WindowsEventConfig)
	case cfg.PatternConfig != nil:
		s, err = newPatternStage(slogger, *cfg.PatternConfig)
		if err != nil {
			return nil, err
		}
	case cfg.TruncateConfig != nil:
		s, err = newTruncateStage(slogger, *cfg.TruncateConfig, registerer)
		if err != nil {
			return nil, err
		}
	default:
		panic(fmt.Sprintf("unreachable; should have decoded into one of the StageConfig fields: %+v", cfg))
	}

	return s, nil
}

// Cleanup implements Stage.
func (*stageProcessor) Cleanup() {
	// no-op
}
