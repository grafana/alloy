package stages

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

var (
	truncateLineField               = "line"
	truncateLabelField              = "label"
	truncateStructuredMetadataField = "structured_metadata"
	truncateExtractedField          = "extracted"
)

const (
	errLimitMustBeGreaterThanZero = "limit must be greater than zero"
	errSourcesForLine             = "sources cannot be set when source_type is 'line'"
	errAtLeastOneRule             = "at least one truncate rule must be defined"
)

// TruncateConfig contains the configuration for a truncateStage
type TruncateConfig struct {
	Rules []*RuleConfig `alloy:"rule,block"`
}

type RuleConfig struct {
	Limit      units.Base2Bytes   `alloy:"limit,attr"`
	Suffix     string             `alloy:"suffix,attr,optional"`
	Sources    []string           `alloy:"sources,attr,optional"`
	SourceType TruncateSourceType `alloy:"source_type,attr,optional"`

	effectiveLimit units.Base2Bytes
}

type TruncateSourceType string

const (
	TruncateSourceLine               TruncateSourceType = "line"
	TruncateSourceLabel              TruncateSourceType = "label"
	TruncateSourceStructuredMetadata TruncateSourceType = "structured_metadata"
	TruncateSourceExtractedMap       TruncateSourceType = "extracted"
)

// validateTruncateConfig validates the TruncateConfig for the truncateStage
func validateTruncateConfig(cfg *TruncateConfig) error {
	if len(cfg.Rules) == 0 {
		return errors.New(errAtLeastOneRule)
	}

	for _, r := range cfg.Rules {
		r.effectiveLimit = r.Limit

		if r.Limit <= 0 {
			return errors.New(errLimitMustBeGreaterThanZero)
		}

		if r.SourceType == "" {
			r.SourceType = TruncateSourceLine
		}

		if r.SourceType == TruncateSourceLine && len(r.Sources) > 0 {
			return errors.New(errSourcesForLine)
		}

		if len(r.Suffix) > 0 {
			if len(r.Suffix) >= int(r.Limit) {
				return errors.New("suffix length cannot be greater than or equal to limit")
			}

			r.effectiveLimit -= units.Base2Bytes(len(r.Suffix))
		}
	}

	return nil
}

// newTruncateStage creates a TruncateStage from config
func newTruncateStage(logger log.Logger, config TruncateConfig, registerer prometheus.Registerer) (Stage, error) {
	err := validateTruncateConfig(&config)
	if err != nil {
		return nil, err
	}

	return &truncateStage{
		logger:         log.With(logger, "component", "stage", "type", "truncate"),
		cfg:            &config,
		truncatedCount: getTruncateCountMetric(registerer),
	}, nil
}

// truncateStage applies Label matchers to determine if the include stages should be run
type truncateStage struct {
	logger         log.Logger
	cfg            *TruncateConfig
	truncatedCount *prometheus.CounterVec
}

func (m *truncateStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			truncated := map[string]struct{}{}
			for _, r := range m.cfg.Rules {
				switch r.SourceType {
				case TruncateSourceLine:
					if len(e.Line) > int(r.effectiveLimit) {
						e.Line = e.Line[:r.effectiveLimit] + r.Suffix
						markTruncated(m.truncatedCount, truncated, truncateLineField)

						if Debug {
							level.Debug(m.logger).Log("msg", "line has been truncated", "limit", r.effectiveLimit, "truncated_line", e.Line)
						}
					}
				case TruncateSourceLabel:
					if len(r.Sources) > 0 {
						for _, source := range r.Sources {
							name := model.LabelName(source)
							if v, ok := e.Labels[name]; ok {
								m.tryTruncateLabel(r, e.Labels, name, v, truncated)
							}
						}
					} else {
						for k, v := range e.Labels {
							m.tryTruncateLabel(r, e.Labels, k, v, truncated)
						}
					}
				case TruncateSourceStructuredMetadata:
					if len(r.Sources) > 0 {
						for i, v := range e.StructuredMetadata {
							if slices.Contains(r.Sources, v.Name) {
								// Returns unmodified if no truncation was required
								e.StructuredMetadata[i] = m.tryTruncateStructuredMetadata(r, v, truncated)
							}
						}
					} else {
						for i, v := range e.StructuredMetadata {
							// Returns unmodified if no truncation was required
							e.StructuredMetadata[i] = m.tryTruncateStructuredMetadata(r, v, truncated)
						}
					}
				case TruncateSourceExtractedMap:
					if len(r.Sources) > 0 {
						for _, source := range r.Sources {
							if v, ok := e.Extracted[source]; ok {
								m.tryTruncateExtracted(r, e.Extracted, source, v, truncated)
							}
						}
					} else {
						for k, v := range e.Extracted {
							m.tryTruncateExtracted(r, e.Extracted, k, v, truncated)
						}
					}
				}
				if len(truncated) > 0 {
					// Ensure that we properly support multiple stages truncating different fields
					if existing, ok := e.Extracted["truncated"]; ok {
						if strExisting, ok := existing.(string); ok {
							for s := range strings.SplitSeq(strExisting, ",") {
								truncated[s] = struct{}{}
							}
						}
					}
					e.Extracted["truncated"] = strings.Join(slices.Sorted(maps.Keys(truncated)), ",")
				}
			}
			out <- e
		}
	}()
	return out
}

func (m *truncateStage) tryTruncateLabel(rule *RuleConfig, l model.LabelSet, name model.LabelName, val model.LabelValue, truncated map[string]struct{}) {
	if len(val) > int(rule.effectiveLimit) {
		l[name] = val[:rule.effectiveLimit] + model.LabelValue(rule.Suffix)
		markTruncated(m.truncatedCount, truncated, truncateLabelField)

		if Debug {
			level.Debug(m.logger).Log("msg", "label has been truncated", "limit", rule.effectiveLimit, "name", name, "truncated_value", l[name])
		}
	}
}

func (m *truncateStage) tryTruncateExtracted(rule *RuleConfig, extracted map[string]any, name string, val any, truncated map[string]struct{}) {
	if strVal, ok := val.(string); ok && len(strVal) > int(rule.effectiveLimit) {
		extracted[name] = strVal[:rule.effectiveLimit] + rule.Suffix
		markTruncated(m.truncatedCount, truncated, truncateExtractedField)

		if Debug {
			level.Debug(m.logger).Log("msg", "extracted has been truncated", "limit", rule.effectiveLimit, "name", name, "truncated_value", extracted[name])
		}
	}
}

func (m *truncateStage) tryTruncateStructuredMetadata(rule *RuleConfig, metadata push.LabelAdapter, truncated map[string]struct{}) push.LabelAdapter {
	if len(metadata.Value) > int(rule.effectiveLimit) {
		metadata.Value = metadata.Value[:rule.effectiveLimit] + rule.Suffix
		markTruncated(m.truncatedCount, truncated, truncateStructuredMetadataField)

		if Debug {
			level.Debug(m.logger).Log("msg", "structured_metadata has been truncated", "limit", rule.effectiveLimit, "name", metadata.Name, "truncated_value", metadata.Value)
		}
	}
	return metadata
}

func markTruncated(metric *prometheus.CounterVec, set map[string]struct{}, field string) {
	metric.WithLabelValues(field).Inc()
	set[field] = struct{}{}
}

// Name implements Stage
func (m *truncateStage) Name() string {
	return StageTypeTruncate
}

// Cleanup implements Stage.
func (*truncateStage) Cleanup() {
	// no-op
}

func getTruncateCountMetric(registerer prometheus.Registerer) *prometheus.CounterVec {
	truncateCount := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_process_truncated_fields_total",
		Help: "A count of all log lines, labels, extracted values, or structured_metadata truncated as a result of a pipeline stage",
	}, []string{"field"})
	return util.MustRegisterOrGet(registerer, truncateCount).(*prometheus.CounterVec)
}
