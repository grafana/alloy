package stages

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"golang.org/x/time/rate"
)

// Configuration errors.
var (
	ErrLimitStageInvalidRateOrBurst = errors.New("limit stage failed to parse rate or burst")
	ErrLimitStageByLabelMustDrop    = errors.New("when ratelimiting by label, drop must be true")
	ratelimitDropReason             = "ratelimit_drop_stage"
)

// MinReasonableMaxDistinctLabels provides a sensible default.
const MinReasonableMaxDistinctLabels = 10000 // 80bytes per rate.Limiter ~ 1MiB memory

// LimitConfig sets up a Limit stage.
type LimitConfig struct {
	Rate              float64 `alloy:"rate,attr"`
	Burst             int     `alloy:"burst,attr"`
	Drop              bool    `alloy:"drop,attr,optional"`
	ByLabelName       string  `alloy:"by_label_name,attr,optional"`
	MaxDistinctLabels int     `alloy:"max_distinct_labels,attr,optional"`
}

func newLimitStage(logger log.Logger, cfg LimitConfig, registerer prometheus.Registerer) (Stage, error) {
	err := validateLimitConfig(cfg)
	if err != nil {
		return nil, err
	}

	logger = log.With(logger, "component", "stage", "type", "limit")
	if cfg.ByLabelName != "" && cfg.MaxDistinctLabels < MinReasonableMaxDistinctLabels {
		level.Warn(logger).Log(
			"msg",
			fmt.Sprintf("max_distinct_labels was adjusted up to the minimal reasonable value of %d", MinReasonableMaxDistinctLabels),
		)
		cfg.MaxDistinctLabels = MinReasonableMaxDistinctLabels
	}

	r := &limitStage{
		logger:    logger,
		cfg:       cfg,
		dropCount: getDropCountMetric(registerer),
	}

	if cfg.ByLabelName != "" {
		r.dropCountByLabel = getDropCountByLabelMetric(registerer)
		newRateLimiter := func() *rate.Limiter { return rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst) }
		gcCb := func() { r.dropCountByLabel.Reset() }
		r.rateLimiterByLabel = NewGenMap[model.LabelValue, *rate.Limiter](cfg.MaxDistinctLabels, newRateLimiter, gcCb)
	} else {
		r.rateLimiter = rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst)
	}

	return r, nil
}

func validateLimitConfig(cfg LimitConfig) error {
	if cfg.Rate <= 0 || cfg.Burst <= 0 {
		return ErrLimitStageInvalidRateOrBurst
	}

	if cfg.ByLabelName != "" && !cfg.Drop {
		return ErrLimitStageByLabelMustDrop
	}
	return nil
}

// limitStage applies Label matchers to determine if the include stages should be run
type limitStage struct {
	logger             log.Logger
	cfg                LimitConfig
	rateLimiter        *rate.Limiter
	rateLimiterByLabel GenerationalMap[model.LabelValue, *rate.Limiter]
	dropCount          *prometheus.CounterVec
	dropCountByLabel   *prometheus.CounterVec
}

func (m *limitStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			if !m.shouldThrottle(e.Labels) {
				out <- e
				continue
			}
		}
	}()
	return out
}

func (m *limitStage) shouldThrottle(labels model.LabelSet) bool {
	if m.cfg.ByLabelName != "" {
		labelValue, ok := labels[model.LabelName(m.cfg.ByLabelName)]
		if !ok {
			return false // if no label found, dont ratelimit
		}
		rl := m.rateLimiterByLabel.GetOrCreate(labelValue)
		if rl.Allow() {
			return false
		}
		m.dropCount.WithLabelValues(ratelimitDropReason).Inc()
		m.dropCountByLabel.WithLabelValues(m.cfg.ByLabelName, string(labelValue)).Inc()
		return true
	}

	if m.cfg.Drop {
		if m.rateLimiter.Allow() {
			return false
		}
		m.dropCount.WithLabelValues(ratelimitDropReason).Inc()
		return true
	}
	_ = m.rateLimiter.Wait(context.Background())
	return false
}

// Cleanup implements Stage.
func (*limitStage) Cleanup() {
	// no-op
}

func getDropCountByLabelMetric(registerer prometheus.Registerer) *prometheus.CounterVec {
	return registerCounterVec(registerer, "loki_process", "dropped_lines_by_label_total",
		"A count of all log lines dropped as a result of a pipeline stage",
		[]string{"label_name", "label_value"})
}

// GenerationalMap is ported from Loki's pkg/util package. It didn't exist
// in our dependency at the time, so I copied the implementation over.
type GenerationalMap[K comparable, V any] struct {
	oldgen map[K]V
	newgen map[K]V

	maxSize int
	newV    func() V
	gcCb    func()
}

// NewGenMap created which maintains at most maxSize recently used entries
func NewGenMap[K comparable, V any](maxSize int, newV func() V, gcCb func()) GenerationalMap[K, V] {
	return GenerationalMap[K, V]{
		newgen:  make(map[K]V),
		maxSize: maxSize,
		newV:    newV,
		gcCb:    gcCb,
	}
}

func (m *GenerationalMap[K, T]) GetOrCreate(key K) T {
	v, ok := m.newgen[key]
	if !ok {
		if v, ok = m.oldgen[key]; !ok {
			v = m.newV()
		}
		m.newgen[key] = v

		if len(m.newgen) == m.maxSize {
			m.oldgen = m.newgen
			m.newgen = make(map[K]T)
			if m.gcCb != nil {
				m.gcCb()
			}
		}
	}
	return v
}

func registerCounterVec(registerer prometheus.Registerer, namespace, name, help string, labels []string) *prometheus.CounterVec {
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      name,
		Help:      help,
	}, labels)
	err := registerer.Register(vec)
	if err != nil {
		if existing, ok := err.(prometheus.AlreadyRegisteredError); ok {
			vec = existing.ExistingCollector.(*prometheus.CounterVec)
		} else {
			// Same behavior as MustRegister if the error is not for AlreadyRegistered
			panic(err)
		}
	}
	return vec
}
