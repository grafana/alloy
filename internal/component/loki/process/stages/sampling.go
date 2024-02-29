package stages

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/uber/jaeger-client-go/utils"
)

const (
	ErrSamplingStageInvalidRate = "sampling stage failed to parse rate,Sampling Rate must be between 0.0 and 1.0, received %f"
)
const maxRandomNumber = ^(uint64(1) << 63) // i.e. 0x7fffffffffffffff

var (
	defaultSamplingpReason = "sampling_stage"
)

// SamplingConfig contains the configuration for a samplingStage
type SamplingConfig struct {
	DropReason   *string `river:"drop_counter_reason,attr,optional"`
	SamplingRate float64 `river:"rate,attr"`
}

func (s *SamplingConfig) SetToDefault() {
	if s.DropReason == nil || *s.DropReason == "" {
		s.DropReason = &defaultSamplingpReason
	}
}

func (s *SamplingConfig) Validate() error {
	if s.SamplingRate < 0.0 || s.SamplingRate > 1.0 {
		return fmt.Errorf(ErrSamplingStageInvalidRate, s.SamplingRate)
	}
	return nil
}

// newSamplingStage creates a SamplingStage from config
// code from jaeger project.
// github.com/uber/jaeger-client-go@v2.30.0+incompatible/tracer.go:126
func newSamplingStage(logger log.Logger, cfg SamplingConfig, registerer prometheus.Registerer) Stage {
	samplingRate := math.Max(0.0, math.Min(cfg.SamplingRate, 1.0))
	samplingBoundary := uint64(float64(maxRandomNumber) * samplingRate)
	seedGenerator := utils.NewRand(time.Now().UnixNano())
	source := rand.NewSource(seedGenerator.Int63())
	return &samplingStage{
		logger:           log.With(logger, "component", "stage", "type", "sampling"),
		cfg:              cfg,
		dropCount:        getDropCountMetric(registerer),
		samplingBoundary: samplingBoundary,
		source:           source,
	}
}

type samplingStage struct {
	logger           log.Logger
	cfg              SamplingConfig
	dropCount        *prometheus.CounterVec
	samplingBoundary uint64
	source           rand.Source
}

func (m *samplingStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			if m.isSampled() {
				out <- e
				continue
			}
			m.dropCount.WithLabelValues(*m.cfg.DropReason).Inc()
		}
	}()
	return out
}

// code from jaeger project.
// github.com/uber/jaeger-client-go@v2.30.0+incompatible/sampler.go:144
// func (s *ProbabilisticSampler) IsSampled(id TraceID, operation string) (bool, []Tag)
func (m *samplingStage) isSampled() bool {
	return m.samplingBoundary >= m.randomID()&maxRandomNumber
}
func (m *samplingStage) randomID() uint64 {
	val := m.randomNumber()
	for val == 0 {
		val = m.randomNumber()
	}
	return val
}
func (m *samplingStage) randomNumber() uint64 {
	return uint64(m.source.Int63())
}

// Name implements Stage
func (m *samplingStage) Name() string {
	return StageTypeSampling
}
