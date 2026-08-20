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

// Stopper is an optional interface for stages that need an out-of-band signal
// to unblock goroutines during shutdown. Implementations must not block,
// panic, or assume Run has stopped.
type Stopper interface {
	Stop()
}

// newStage creates a new stage for the given type and configuration.
func newStage(slogger *slog.Logger, cfg StageConfig, registerer prometheus.Registerer, minStability featuregate.Stability) (Stage, error) {
	return newStageWithNextFn(slogger, cfg, registerer, minStability, nil)
}

func newStageWithNextFn(
	slogger *slog.Logger,
	cfg StageConfig,
	registerer prometheus.Registerer,
	minStability featuregate.Stability,
	next NextFn,
) (Stage, error) {

	var (
		s   Stage
		err error
	)
	switch {
	case cfg.DockerConfig != nil:
		s = newDockerStage(slogger, registerer, minStability, next)
	case cfg.CRIConfig != nil:
		s = newCRIStage(slogger, *cfg.CRIConfig, registerer, minStability, next)
	case cfg.JSONConfig != nil:
		s, err = newJSONStage(slogger, *cfg.JSONConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.LogfmtConfig != nil:
		s, err = newLogfmtStage(slogger, *cfg.LogfmtConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.LuhnFilterConfig != nil:
		s, err = newLuhnFilterStage(*cfg.LuhnFilterConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.MetricsConfig != nil:
		s, err = newMetricStage(slogger, *cfg.MetricsConfig, registerer)
		if err != nil {
			return nil, err
		}
	case cfg.LabelsConfig != nil:
		s, err = newLabelStage(slogger, *cfg.LabelsConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadata != nil:
		s, err = newStructuredMetadataStage(slogger, *cfg.StructuredMetadata, next)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadataDropConfig != nil:
		s, err = newStructuredMetadataDropStage(slogger, *cfg.StructuredMetadataDropConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.RegexConfig != nil:
		s, err = newRegexStage(slogger, *cfg.RegexConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.TimestampConfig != nil:
		s, err = newTimestampStage(slogger, *cfg.TimestampConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.OutputConfig != nil:
		s, err = newOutputStage(slogger, *cfg.OutputConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.MatchConfig != nil:
		s, err = newMatcherStage(slogger, *cfg.MatchConfig, registerer, minStability)
		if err != nil {
			return nil, err
		}
	case cfg.TemplateConfig != nil:
		s, err = newTemplateStage(slogger, *cfg.TemplateConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.TenantConfig != nil:
		s, err = newTenantStage(slogger, *cfg.TenantConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.ReplaceConfig != nil:
		s, err = newReplaceStage(slogger, *cfg.ReplaceConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.LimitConfig != nil:
		s, err = newLimitStage(slogger, *cfg.LimitConfig, registerer)
		if err != nil {
			return nil, err
		}
	case cfg.DropConfig != nil:
		s, err = newDropStage(slogger, *cfg.DropConfig, registerer, next)
		if err != nil {
			return nil, err
		}
	case cfg.MultilineConfig != nil:
		s, err = newMultilineStage(slogger, *cfg.MultilineConfig)
		if err != nil {
			return nil, err
		}
	case cfg.PackConfig != nil:
		s, err = newPackStage(slogger, *cfg.PackConfig, registerer, next)
		if err != nil {
			return nil, err
		}
	case cfg.LabelKeepConfig != nil:
		s, err = newLabelKeepStage(*cfg.LabelKeepConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.LabelDropConfig != nil:
		s, err = newLabelDropStage(*cfg.LabelDropConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.StaticLabelsConfig != nil:
		s, err = newStaticLabelsStage(*cfg.StaticLabelsConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.GeoIPConfig != nil:
		s, err = newGeoIPStage(slogger, *cfg.GeoIPConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.DecolorizeConfig != nil:
		s = newDecolorizeStage(*cfg.DecolorizeConfig, next)
	case cfg.SamplingConfig != nil:
		s, err = newSamplingStage(slogger, *cfg.SamplingConfig, registerer, next)
		if err != nil {
			return nil, err
		}
	case cfg.EventLogMessageConfig != nil:
		s = newEventLogMessageStage(slogger, cfg.EventLogMessageConfig, next)
	case cfg.WindowsEventConfig != nil:
		s = newWindowsEventStage(slogger, cfg.WindowsEventConfig, next)
	case cfg.PatternConfig != nil:
		s, err = newPatternStage(slogger, *cfg.PatternConfig, next)
		if err != nil {
			return nil, err
		}
	case cfg.TruncateConfig != nil:
		s = newTruncateStage(slogger, *cfg.TruncateConfig, registerer, next)
	case cfg.SplitJSONConfig != nil:
		s = newSplitJSONStage(slogger, *cfg.SplitJSONConfig, next)
	default:
		panic(fmt.Sprintf("unreachable; should have decoded into one of the StageConfig fields: %+v", cfg))
	}

	return s, nil
}
