package stages

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

const (
	errTruncateStageEmptyConfig = "truncate stage config must contain at least one of `entry_limit`, `label_limit`, or `structured_metadata_limit`"
)

var (
	truncateLineField               = "line"
	truncateLabelField              = "label"
	truncateStructuredMetadataField = "structured_metadata"
)

// TruncateConfig contains the configuration for a truncateStage
type TruncateConfig struct {
	LineLimit               units.Base2Bytes `alloy:"line_limit,attr,optional"`
	LabelLimit              units.Base2Bytes `alloy:"label_limit,attr,optional"`
	StructuredMetadataLimit units.Base2Bytes `alloy:"structured_metadata_limit,attr,optional"`
	Suffix                  string           `alloy:"suffix,attr,optional"`

	effectiveLineLimit               units.Base2Bytes
	effectiveLabelLimit              units.Base2Bytes
	effectiveStructuredMetadataLimit units.Base2Bytes
}

// validateTruncateConfig validates the TruncateConfig for the truncateStage
func validateTruncateConfig(cfg *TruncateConfig) error {
	if cfg == nil ||
		(cfg.LineLimit == 0 && cfg.LabelLimit == 0 && cfg.StructuredMetadataLimit == 0) {

		return errors.New(errTruncateStageEmptyConfig)
	}

	cfg.effectiveLineLimit = cfg.LineLimit
	cfg.effectiveLabelLimit = cfg.LabelLimit
	cfg.effectiveStructuredMetadataLimit = cfg.StructuredMetadataLimit

	var errs error
	if len(cfg.Suffix) > 0 {
		if len(cfg.Suffix) >= int(cfg.LineLimit) && cfg.LineLimit > 0 {
			errs = errors.Join(errs, errors.New("suffix length cannot be greater than or equal to line_limit"))
		}
		if len(cfg.Suffix) >= int(cfg.LabelLimit) && cfg.LabelLimit > 0 {
			errs = errors.Join(errs, errors.New("suffix length cannot be greater than or equal to label_limit"))
		}
		if len(cfg.Suffix) >= int(cfg.StructuredMetadataLimit) && cfg.StructuredMetadataLimit > 0 {
			errs = errors.Join(errs, errors.New("suffix length cannot be greater than or equal to structured_metadata_limit"))
		}
		if errs != nil {
			return errs
		}

		if cfg.LineLimit > 0 {
			cfg.effectiveLineLimit -= units.Base2Bytes(len(cfg.Suffix))
		}
		if cfg.LabelLimit > 0 {
			cfg.effectiveLabelLimit -= units.Base2Bytes(len(cfg.Suffix))
		}
		if cfg.StructuredMetadataLimit > 0 {
			cfg.effectiveStructuredMetadataLimit -= units.Base2Bytes(len(cfg.Suffix))
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
			truncated := map[string]any{}
			if m.cfg.LineLimit > 0 && len(e.Line) > int(m.cfg.effectiveLineLimit) {
				e.Line = e.Line[:m.cfg.effectiveLineLimit] + m.cfg.Suffix
				m.truncatedCount.WithLabelValues(truncateLineField).Inc()
				truncated[truncateLineField] = true

				if Debug {
					level.Debug(m.logger).Log("msg", "line has been truncated", "limit", m.cfg.effectiveLineLimit, "truncated_line", e.Line)
				}
			}
			if m.cfg.LabelLimit > 0 {
				for k, v := range e.Labels {
					if len(v) > int(m.cfg.effectiveLabelLimit) {
						e.Labels[k] = v[:m.cfg.effectiveLabelLimit] + model.LabelValue(m.cfg.Suffix)
						m.truncatedCount.WithLabelValues(truncateLabelField).Inc()
						truncated[truncateLabelField] = true

						if Debug {
							level.Debug(m.logger).Log("msg", "label has been truncated", "limit", m.cfg.effectiveLabelLimit, "truncated_label", e.Labels[k])
						}
					}
				}
			}
			if m.cfg.StructuredMetadataLimit > 0 && e.StructuredMetadata != nil {
				for i, v := range e.StructuredMetadata {
					if len(v.Value) > int(m.cfg.effectiveStructuredMetadataLimit) {
						v.Value = v.Value[:m.cfg.effectiveStructuredMetadataLimit] + m.cfg.Suffix
						e.StructuredMetadata[i] = v
						m.truncatedCount.WithLabelValues(truncateStructuredMetadataField).Inc()
						truncated[truncateStructuredMetadataField] = true

						if Debug {
							level.Debug(m.logger).Log("msg", "structured_metadata has been truncated", "limit", m.cfg.effectiveStructuredMetadataLimit, "truncated_structured_metadata", v.Value)
						}
					}
				}
			}
			if len(truncated) > 0 {
				e.Extracted["truncated"] = strings.Join(slices.Sorted(maps.Keys(truncated)), ",")
			}
			out <- e
		}
	}()
	return out
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
		Help: "A count of all log lines, labels, or structured_metadata truncated as a result of a pipeline stage",
	}, []string{"field"})
	return util.MustRegisterOrGet(registerer, truncateCount).(*prometheus.CounterVec)
}
