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
		s, err = NewDocker(opts.slogger, opts.registerer, opts.minStability)
		if err != nil {
			return nil, err
		}
	case cfg.CRIConfig != nil:
		s = newCRIStage(*cfg.CRIConfig, opts)
	case cfg.JSONConfig != nil:
		s, err = newJSONStage(opts.slogger, *cfg.JSONConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LogfmtConfig != nil:
		s, err = newLogfmtStage(opts.slogger, *cfg.LogfmtConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LuhnFilterConfig != nil:
		s, err = newLuhnFilterStage(*cfg.LuhnFilterConfig)
		if err != nil {
			return nil, err
		}
	case cfg.MetricsConfig != nil:
		s, err = newMetricStage(opts.slogger, *cfg.MetricsConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
	case cfg.LabelsConfig != nil:
		s, err = newLabelStage(opts.slogger, *cfg.LabelsConfig)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadata != nil:
		s, err = newStructuredMetadataStage(opts.slogger, *cfg.StructuredMetadata)
		if err != nil {
			return nil, err
		}
	case cfg.StructuredMetadataDropConfig != nil:
		s, err = newStructuredMetadataDropStage(opts.slogger, *cfg.StructuredMetadataDropConfig)
		if err != nil {
			return nil, err
		}
	case cfg.RegexConfig != nil:
		s, err = newRegexStage(opts.slogger, *cfg.RegexConfig)
		if err != nil {
			return nil, err
		}
	case cfg.TimestampConfig != nil:
		s, err = newTimestampStage(opts.slogger, *cfg.TimestampConfig)
		if err != nil {
			return nil, err
		}
	case cfg.OutputConfig != nil:
		s, err = newOutputStage(opts.slogger, *cfg.OutputConfig)
		if err != nil {
			return nil, err
		}
	case cfg.MatchConfig != nil:
		s, err = newMatcherStage(opts.slogger, *cfg.MatchConfig, opts.registerer, opts.minStability)
		if err != nil {
			return nil, err
		}
	case cfg.TemplateConfig != nil:
		s, err = newTemplateStage(opts.slogger, *cfg.TemplateConfig)
		if err != nil {
			return nil, err
		}
	case cfg.TenantConfig != nil:
		s, err = newTenantStage(opts.slogger, *cfg.TenantConfig)
		if err != nil {
			return nil, err
		}
	case cfg.ReplaceConfig != nil:
		s, err = newReplaceStage(opts.slogger, *cfg.ReplaceConfig)
		if err != nil {
			return nil, err
		}
	case cfg.LimitConfig != nil:
		s, err = newLimitStage(opts.slogger, *cfg.LimitConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
	case cfg.DropConfig != nil:
		s, err = newDropStage(opts.slogger, *cfg.DropConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
	case cfg.MultilineConfig != nil:
		s, err = newMultilineStage(opts.slogger, *cfg.MultilineConfig)
		if err != nil {
			return nil, err
		}
	case cfg.PackConfig != nil:
		s, err = newPackStage(opts.slogger, *cfg.PackConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
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
		s, err = newGeoIPStage(opts.slogger, *cfg.GeoIPConfig)
		if err != nil {
			return nil, err
		}
	case cfg.DecolorizeConfig != nil:
		s, err = newDecolorizeStage(*cfg.DecolorizeConfig)
		if err != nil {
			return nil, err
		}
	case cfg.SamplingConfig != nil:
		s, err = newSamplingStage(opts.slogger, *cfg.SamplingConfig, opts.registerer)
		if err != nil {
			return nil, err
		}
	case cfg.EventLogMessageConfig != nil:
		s = newEventLogMessageStage(opts.slogger, cfg.EventLogMessageConfig)
	case cfg.WindowsEventConfig != nil:
		s = newWindowsEventStage(opts.slogger, cfg.WindowsEventConfig)
	case cfg.PatternConfig != nil:
		s, err = newPatternStage(opts.slogger, *cfg.PatternConfig)
		if err != nil {
			return nil, err
		}
	case cfg.TruncateConfig != nil:
		s = newTruncateStage(opts.slogger, *cfg.TruncateConfig, opts.registerer)
	case cfg.SplitJSONConfig != nil:
		s = newSplitJSONStage(opts.slogger, *cfg.SplitJSONConfig)
	default:
		panic(fmt.Sprintf("unreachable; should have decoded into one of the StageConfig fields: %+v", cfg))
	}

	return s, nil
}

// Cleanup implements Stage.
func (*stageProcessor) Cleanup() {
	// no-op
}
