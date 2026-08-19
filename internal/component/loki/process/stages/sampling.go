package stages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/sampling"
)

const (
	errSamplingStageInvalidRate = "sampling stage failed to parse rate,Sampling Rate must be between 0.0 and 1.0, received %f"
)

// SamplingConfig contains the configuration for a samplingStage
type SamplingConfig struct {
	DropReason   string  `alloy:"drop_counter_reason,attr,optional"`
	SamplingRate float64 `alloy:"rate,attr"`
}

func (s *SamplingConfig) SetToDefault() {
	s.DropReason = "sampling_stage"
}

func (s *SamplingConfig) Validate() error {
	if err := sampling.ValidateRate(s.SamplingRate); err != nil {
		return fmt.Errorf(errSamplingStageInvalidRate, s.SamplingRate)
	}
	return nil
}

var (
	_ Stage = (*samplingStage)(nil)
	_ stage = (*samplingStage)(nil)
)

// newSamplingStage creates a SamplingStage from config using the shared probabilistic sampler.
func newSamplingStage(logger *slog.Logger, cfg SamplingConfig, registerer prometheus.Registerer, next NextFn) (*samplingStage, error) {
	dropCount, err := getDropCountMetric(registerer)
	if err != nil {
		return nil, err
	}

	return &samplingStage{
		next:      next,
		logger:    logger.With("stage", "sampling"),
		sampler:   sampling.NewSampler(cfg.SamplingRate),
		dropCount: dropCount.WithLabelValues(cfg.DropReason),
	}, nil
}

type samplingStage struct {
	next      NextFn
	logger    *slog.Logger
	sampler   *sampling.Sampler
	dropCount prometheus.Counter
}

func (m *samplingStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			if m.sampler.ShouldSample() {
				out <- e
				continue
			}
			m.dropCount.Inc()
		}
	}()
	return out
}

// process implements stage.
func (m *samplingStage) process(ctx context.Context, entries []Entry) error {
	var dst int
	for _, e := range entries {
		if !m.sampler.ShouldSample() {
			m.dropCount.Inc()
			continue
		}
		entries[dst] = e
		dst++
	}

	if dst == 0 {
		return nil
	}

	return m.next(ctx, entries[:dst])
}

// Cleanup implements Stage.
func (*samplingStage) Cleanup() {}
