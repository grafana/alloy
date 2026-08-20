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
	return newStageWithOpts(cfg, stageOpts{
		slogger:      slogger,
		registerer:   registerer,
		minStability: minStability,
	})
}

type stageOpts struct {
	slogger      *slog.Logger
	registerer   prometheus.Registerer
	minStability featuregate.Stability

	next nextFn
}

func newStageWithOpts(
	cfg StageConfig,
	opts stageOpts,
) (Stage, error) {

	var (
		s   Stage
		err error
	)
	switch {
	case cfg.DockerConfig != nil:
		s = newDockerStage(opts)
	case cfg.CRIConfig != nil:
		s = newCRIStage(*cfg.CRIConfig, opts)
	case cfg.JSONConfig != nil:
		s, err = newJSONStage(*cfg.JSONConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.LogfmtConfig != nil:
		s, err = newLogfmtStage(*cfg.LogfmtConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.LuhnFilterConfig != nil:
		s, err = newLuhnFilterStage(*cfg.LuhnFilterConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.MetricsConfig != nil:
		s, err = newMetricStage(opts.slogger, *cfg.MetricsConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
	case cfg.LabelsConfig != nil:
		s, err = newLabelStage(*cfg.LabelsConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadata != nil:
		s, err = newStructuredMetadataStage(*cfg.StructuredMetadata, opts)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadataDropConfig != nil:
		s, err = newStructuredMetadataDropStage(*cfg.StructuredMetadataDropConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.RegexConfig != nil:
		s, err = newRegexStage(*cfg.RegexConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.TimestampConfig != nil:
		s, err = newTimestampStage(*cfg.TimestampConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.OutputConfig != nil:
		s, err = newOutputStage(*cfg.OutputConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.MatchConfig != nil:
		s, err = newMatcherStage(opts.slogger, *cfg.MatchConfig, opts.registerer, opts.minStability)
		if err != nil {
			return nil, err
		}
	case cfg.TemplateConfig != nil:
		s, err = newTemplateStage(*cfg.TemplateConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.TenantConfig != nil:
		s, err = newTenantStage(*cfg.TenantConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.ReplaceConfig != nil:
		s, err = newReplaceStage(*cfg.ReplaceConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.LimitConfig != nil:
		s, err = newLimitStage(opts.slogger, *cfg.LimitConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
	case cfg.DropConfig != nil:
		s, err = newDropStage(*cfg.DropConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.MultilineConfig != nil:
		s, err = newMultilineStage(opts.slogger, *cfg.MultilineConfig)
		if err != nil {
			return nil, err
		}
	case cfg.PackConfig != nil:
		s, err = newPackStage(*cfg.PackConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.LabelKeepConfig != nil:
		s, err = newLabelKeepStage(*cfg.LabelKeepConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.LabelDropConfig != nil:
		s, err = newLabelDropStage(*cfg.LabelDropConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.StaticLabelsConfig != nil:
		s, err = newStaticLabelsStage(*cfg.StaticLabelsConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.GeoIPConfig != nil:
		s, err = newGeoIPStage(*cfg.GeoIPConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.DecolorizeConfig != nil:
		s = newDecolorizeStage(*cfg.DecolorizeConfig, opts)
	case cfg.SamplingConfig != nil:
		s, err = newSamplingStage(*cfg.SamplingConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.EventLogMessageConfig != nil:
		s = newEventLogMessageStage(cfg.EventLogMessageConfig, opts)
	case cfg.WindowsEventConfig != nil:
		s = newWindowsEventStage(opts.slogger, cfg.WindowsEventConfig)
	case cfg.PatternConfig != nil:
		s, err = newPatternStage(*cfg.PatternConfig, opts)
		if err != nil {
			return nil, err
		}
	case cfg.TruncateConfig != nil:
		s = newTruncateStage(*cfg.TruncateConfig, opts)
	case cfg.SplitJSONConfig != nil:
		s = newSplitJSONStage(*cfg.SplitJSONConfig, opts)
	default:
		panic(fmt.Sprintf("unreachable; should have decoded into one of the StageConfig fields: %+v", cfg))
	}

	return s, nil
}
