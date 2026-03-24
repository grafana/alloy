package testcomponent

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.slow_update",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      SlowUpdateConfig{},
		Exports:   SlowUpdateExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewSlowUpdate(opts)
		},
	})
}

type SlowUpdateConfig struct {
	UpdateLag time.Duration `alloy:"update_lag,attr"`
	Counter   int           `alloy:"counter,attr"` // used to trigger updates
}

type SlowUpdateExports struct{}

type SlowUpdate struct {
	opts      component.Options
	gauge     prometheus.Gauge
	mutex     sync.Mutex
	updateLag time.Duration
}

func NewSlowUpdate(o component.Options) (*SlowUpdate, error) {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "slow_update_counter",
			Help: "Current value of the counter in the slow_update component",
		},
	)

	if err := o.Registerer.Register(gauge); err != nil {
		return nil, err
	}

	s := &SlowUpdate{
		opts:  o,
		gauge: gauge,
	}
	return s, nil
}

var (
	_ component.Component = (*SlowUpdate)(nil)
)

func (s *SlowUpdate) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *SlowUpdate) Update(args component.Arguments) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	c := args.(SlowUpdateConfig)
	level.Info(s.opts.Logger).Log("msg", "Sleeping on Update()", "duration", c.UpdateLag)
	s.updateLag = c.UpdateLag
	time.Sleep(s.updateLag)
	s.gauge.Set(float64(c.Counter))
	level.Info(s.opts.Logger).Log("msg", "Done sleeping and updated the counter", "counter", c.Counter)
	return nil
}

func (s *SlowUpdate) NotifyClusterChange() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	level.Info(s.opts.Logger).Log("msg", "Sleeping on NotifyClusterChange()", "duration", s.updateLag)
	time.Sleep(s.updateLag)
	level.Info(s.opts.Logger).Log("msg", "Done sleeping on NotifyClusterChange()")
}
