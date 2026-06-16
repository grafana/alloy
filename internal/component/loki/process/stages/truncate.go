package stages

import (
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
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

var (
	errTruncateLimit          = errors.New("limit must be greater than zero")
	errTruncateSourcesForLine = errors.New("sources cannot be set when source_type is 'line'")
	errTruncateSuffixLength   = errors.New("suffix length cannot be greater than or equal to limit")
)

// TruncateConfig contains the configuration for a truncateStage
type TruncateConfig struct {
	Rules []*RuleConfig `alloy:"rule,block"`
}

var _ syntax.Validator = (*TruncateConfig)(nil)

// Validate implements syntax.Validator.
func (c *TruncateConfig) Validate() error {
	for _, rule := range c.Rules {
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type RuleConfig struct {
	Limit      units.Base2Bytes `alloy:"limit,attr"`
	Suffix     string           `alloy:"suffix,attr,optional"`
	Sources    []string         `alloy:"sources,attr,optional"`
	SourceType SourceType       `alloy:"source_type,attr,optional"`
}

var (
	_ syntax.Defaulter = (*RuleConfig)(nil)
	_ syntax.Validator = (*RuleConfig)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (r *RuleConfig) SetToDefault() {
	*r = RuleConfig{SourceType: SourceTypeLine}
}

// Validate implements syntax.Validator.
func (r *RuleConfig) Validate() error {
	if r.Limit <= 0 {
		return errTruncateLimit
	}
	if r.SourceType == SourceTypeLine && len(r.Sources) > 0 {
		return errTruncateSourcesForLine
	}
	if len(r.Suffix) > 0 && int(r.Limit) <= len(r.Suffix) {
		return errTruncateSuffixLength
	}
	return nil
}

// effectiveLimit returns the truncation threshold after accounting for the
// suffix length. Computed on demand so RuleConfig stays a pure parse target.
func (r *RuleConfig) effectiveLimit() units.Base2Bytes {
	return r.Limit - units.Base2Bytes(len(r.Suffix))
}

// newTruncateStage creates a TruncateStage from config
func newTruncateStage(logger *slog.Logger, cfg TruncateConfig, registerer prometheus.Registerer) Stage {
	return &truncateStage{
		logger:         logger.With("stage", "truncate"),
		cfg:            cfg,
		truncatedCount: getTruncateCountMetric(registerer),
	}
}

// truncateStage applies Label matchers to determine if the include stages should be run
type truncateStage struct {
	logger         *slog.Logger
	cfg            TruncateConfig
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
				case SourceTypeLine:
					limit := r.effectiveLimit()
					if len(e.Line) > int(limit) {
						e.Line = e.Line[:limit] + r.Suffix
						markTruncated(m.truncatedCount, truncated, truncateLineField)

						if debugEnabled(m.logger) {
							m.logger.Debug("line has been truncated", "limit", limit, "truncated_line", e.Line)
						}
					}
				case SourceTypeLabel:
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
				case SourceTypeStructuredMetadata:
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
				case SourceTypeExtractedMap:
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
	limit := rule.effectiveLimit()
	if len(val) > int(limit) {
		l[name] = val[:limit] + model.LabelValue(rule.Suffix)
		markTruncated(m.truncatedCount, truncated, truncateLabelField)

		if debugEnabled(m.logger) {
			m.logger.Debug("label has been truncated", "limit", limit, "name", name, "truncated_value", l[name])
		}
	}
}

func (m *truncateStage) tryTruncateExtracted(rule *RuleConfig, extracted map[string]any, name string, val any, truncated map[string]struct{}) {
	limit := rule.effectiveLimit()
	if strVal, ok := val.(string); ok && len(strVal) > int(limit) {
		extracted[name] = strVal[:limit] + rule.Suffix
		markTruncated(m.truncatedCount, truncated, truncateExtractedField)

		if debugEnabled(m.logger) {
			m.logger.Debug("extracted has been truncated", "limit", limit, "name", name, "truncated_value", extracted[name])
		}
	}
}

func (m *truncateStage) tryTruncateStructuredMetadata(rule *RuleConfig, metadata push.LabelAdapter, truncated map[string]struct{}) push.LabelAdapter {
	limit := rule.effectiveLimit()
	if len(metadata.Value) > int(limit) {
		metadata.Value = metadata.Value[:limit] + rule.Suffix
		markTruncated(m.truncatedCount, truncated, truncateStructuredMetadataField)

		if debugEnabled(m.logger) {
			m.logger.Debug("structured_metadata has been truncated", "limit", limit, "name", metadata.Name, "truncated_value", metadata.Value)
		}
	}
	return metadata
}

func markTruncated(metric *prometheus.CounterVec, set map[string]struct{}, field string) {
	metric.WithLabelValues(field).Inc()
	set[field] = struct{}{}
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
