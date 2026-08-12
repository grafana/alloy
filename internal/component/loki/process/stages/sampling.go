package stages

import (
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/sampling"
)

const (
	ErrSamplingStageInvalidRate = "sampling stage failed to parse rate,Sampling Rate must be between 0.0 and 1.0, received %f"
)

var (
	defaultSamplingpReason = "sampling_stage"
)

// SamplingConfig contains the configuration for a samplingStage
type SamplingConfig struct {
	DropReason   string  `alloy:"drop_counter_reason,attr,optional"`
	SamplingRate float64 `alloy:"rate,attr"`
}

func (s *SamplingConfig) SetToDefault() {
	s.DropReason = defaultSamplingpReason
}

func (s *SamplingConfig) Validate() error {
	if err := sampling.ValidateRate(s.SamplingRate); err != nil {
		return fmt.Errorf(ErrSamplingStageInvalidRate, s.SamplingRate)
	}
	return nil
}

// newSamplingStage creates a SamplingStage from config using the shared probabilistic sampler.
func newSamplingStage(logger *slog.Logger, cfg SamplingConfig, registerer prometheus.Registerer) (Stage, error) {
	dropCount, err := getDropCountMetric(registerer)
	if err != nil {
		return nil, err
	}

	return &samplingStage{
		logger:      logger.With("stage", "sampling"),
		cfg:         cfg,
		dropCount:   dropCount,
		dropCounter: dropCount.WithLabelValues(cfg.DropReason),
		sampler:     sampling.NewSampler(cfg.SamplingRate),
	}, nil
}

type samplingStage struct {
	logger      *slog.Logger
	cfg         SamplingConfig
	dropCount   *prometheus.CounterVec
	dropCounter prometheus.Counter
	sampler     *sampling.Sampler
}

func (m *samplingStage) Process(e Entry) (Entry, bool) {
	if m.sampler.ShouldSample() {
		return e, false
	}
	m.dropCounter.Inc()
	return e, true
}

// Cleanup implements Stage.
func (*samplingStage) Cleanup() {
	// no-op
}
