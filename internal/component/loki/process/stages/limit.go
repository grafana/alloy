package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"golang.org/x/time/rate"
)

var (
	errLimitStageInvalidRateOrBurst = errors.New("limit stage failed to parse rate or burst")
	errLimitStageByLabelMustDrop    = errors.New("when ratelimiting by label, drop must be true")
)

const (
	// minReasonableMaxDistinctLabels provides a sensible default.
	minReasonableMaxDistinctLabels = 10000 // 80bytes per rate.Limiter ~ 1MiB memory
	ratelimitDropReason            = "ratelimit_drop_stage"
)

// LimitConfig sets up a Limit stage.
type LimitConfig struct {
	Rate              float64 `alloy:"rate,attr"`
	Burst             int     `alloy:"burst,attr"`
	Drop              bool    `alloy:"drop,attr,optional"`
	ByLabelName       string  `alloy:"by_label_name,attr,optional"`
	MaxDistinctLabels int     `alloy:"max_distinct_labels,attr,optional"`
}

var (
	_ Stage   = (*limitStage)(nil)
	_ Stopper = (*limitStage)(nil)
	_ stage   = (*limitStage)(nil)
	_ stopper = (*limitStage)(nil)
)

func newLimitStage(logger *slog.Logger, cfg LimitConfig, registerer prometheus.Registerer, next NextFn) (*limitStage, error) {
	err := validateLimitConfig(cfg)
	if err != nil {
		return nil, err
	}

	logger = logger.With("stage", "limit")
	if cfg.ByLabelName != "" && cfg.MaxDistinctLabels < minReasonableMaxDistinctLabels {
		logger.Warn(fmt.Sprintf("max_distinct_labels was adjusted up to the minimal reasonable value of %d", minReasonableMaxDistinctLabels))
		cfg.MaxDistinctLabels = minReasonableMaxDistinctLabels
	}

	dropCount, err := getDropCountMetric(registerer)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &limitStage{
		next:      next,
		logger:    logger,
		cfg:       cfg,
		dropCount: dropCount,
		ctx:       ctx,
		cancel:    cancel,
	}

	if cfg.ByLabelName != "" {
		r.dropCountByLabel, err = getDropCountByLabelMetric(registerer)
		if err != nil {
			return nil, err
		}
		newRateLimiter := func() *rate.Limiter { return rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst) }
		gcCb := func() { r.dropCountByLabel.Reset() }
		r.rateLimiterByLabel = newGenMap[model.LabelValue, *rate.Limiter](cfg.MaxDistinctLabels, newRateLimiter, gcCb)
	} else {
		r.rateLimiter = rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst)
	}

	return r, nil
}

func validateLimitConfig(cfg LimitConfig) error {
	if cfg.Rate <= 0 || cfg.Burst <= 0 {
		return errLimitStageInvalidRateOrBurst
	}

	if cfg.ByLabelName != "" && !cfg.Drop {
		return errLimitStageByLabelMustDrop
	}
	return nil
}

// limitStage applies Label matchers to determine if the include stages should be run
type limitStage struct {
	next   NextFn
	logger *slog.Logger
	cfg    LimitConfig

	rateLimiter        *rate.Limiter
	rateLimiterByLabel generationalMap[model.LabelValue, *rate.Limiter]

	dropCount        *prometheus.CounterVec
	dropCountByLabel *prometheus.CounterVec

	ctx    context.Context
	cancel context.CancelFunc
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

// process implements stage.
func (m *limitStage) process(ctx context.Context, entries []Entry) error {
	var dst int

	for _, e := range entries {
		if m.shouldThrottle(e.Labels) {
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

// stop implements stopper.
func (m *limitStage) stop() {
	m.cancel()
}

// Stop implements Stopper
func (m *limitStage) Stop() {
	m.stop()
}

func (m *limitStage) shouldThrottle(labels model.LabelSet) bool {
	if m.cfg.ByLabelName != "" {
		labelValue, ok := labels[model.LabelName(m.cfg.ByLabelName)]
		if !ok {
			return false // if no label found, dont ratelimit
		}
		rl := m.rateLimiterByLabel.getOrCreate(labelValue)
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
	return m.rateLimiter.Wait(m.ctx) != nil
}

// Cleanup implements Stage.
func (*limitStage) Cleanup() {}

// generationalMap is ported from Loki's pkg/util package. It didn't exist
// in our dependency at the time, so I copied the implementation over.
type generationalMap[K comparable, V any] struct {
	mut    *sync.Mutex
	oldgen map[K]V
	newgen map[K]V

	maxSize int
	newV    func() V
	gcCb    func()
}

// newGenMap created which maintains at most maxSize recently used entries
func newGenMap[K comparable, V any](maxSize int, newV func() V, gcCb func()) generationalMap[K, V] {
	return generationalMap[K, V]{
		mut:     &sync.Mutex{},
		newgen:  make(map[K]V),
		maxSize: maxSize,
		newV:    newV,
		gcCb:    gcCb,
	}
}

func (m *generationalMap[K, T]) getOrCreate(key K) T {
	m.mut.Lock()
	defer m.mut.Unlock()
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

func getDropCountByLabelMetric(registerer prometheus.Registerer) (*prometheus.CounterVec, error) {
	return registerCounterVec(registerer, "loki_process", "dropped_lines_by_label_total",
		"A count of all log lines dropped as a result of a pipeline stage",
		[]string{"label_name", "label_value"})
}

func registerCounterVec(registerer prometheus.Registerer, namespace, name, help string, labels []string) (*prometheus.CounterVec, error) {
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
			return nil, err
		}
	}
	return vec, nil
}
