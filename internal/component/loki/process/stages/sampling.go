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
func newSamplingStage(logger *slog.Logger, cfg SamplingConfig, registerer prometheus.Registerer) Stage {
	return &samplingStage{
		logger:    logger.With("stage", "sampling"),
		cfg:       cfg,
		dropCount: getDropCountMetric(registerer),
		sampler:   sampling.NewSampler(cfg.SamplingRate),
	}
}

type samplingStage struct {
	logger    *slog.Logger
	cfg       SamplingConfig
	dropCount *prometheus.CounterVec
	sampler   *sampling.Sampler
}

func (m *samplingStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		counter := m.dropCount.WithLabelValues(m.cfg.DropReason)
		for e := range in {
			if m.sampler.ShouldSample() {
				out <- e
				continue
			}
			counter.Inc()
		}
	}()
	return out
}

// Cleanup implements Stage.
func (*samplingStage) Cleanup() {
	// no-op
}
