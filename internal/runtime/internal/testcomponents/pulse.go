package testcomponents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"
)

// testcomponents.pulse sends the value 1 at the defined frequency for a number of times defined by the max argument.
func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.pulse",
		Stability: featuregate.StabilityPublicPreview,
		Args:      PulseConfig{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewPulse(opts, args.(PulseConfig))
		},
	})
}

type PulseConfig struct {
	Frequency time.Duration `alloy:"frequency,attr"`
	Max       int           `alloy:"max,attr"`
	ForwardTo []IntReceiver `alloy:"forward_to,attr,optional"`
}

type Pulse struct {
	opts component.Options
	log  log.Logger

	cfgMut sync.Mutex
	cfg    PulseConfig
	count  int

	pulseCount prometheus.Counter
}

func NewPulse(o component.Options, cfg PulseConfig) (*Pulse, error) {
	t := &Pulse{
		opts:       o,
		log:        o.Logger,
		pulseCount: prometheus.NewCounter(prometheus.CounterOpts{Name: "pulse_count"}),
	}

	err := o.Registerer.Register(t.pulseCount)
	if err != nil {
		return nil, err
	}

	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Pulse)(nil)
)

func (p *Pulse) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(p.getNextPulse()):
			p.cfgMut.Lock()
			if p.cfg.Max == 0 || p.count < p.cfg.Max {
				for _, r := range p.cfg.ForwardTo {
					r.ReceiveInt(1)
				}
				p.pulseCount.Inc()
				p.count++
			}
			p.cfgMut.Unlock()
		}
	}
}

func (t *Pulse) getNextPulse() time.Duration {
	t.cfgMut.Lock()
	defer t.cfgMut.Unlock()
	return t.cfg.Frequency
}

// Update implements Component.
func (t *Pulse) Update(args component.Arguments) error {
	t.cfgMut.Lock()
	defer t.cfgMut.Unlock()

	cfg := args.(PulseConfig)
	if cfg.Frequency == 0 {
		return fmt.Errorf("frequency must not be 0")
	}

	level.Info(t.log).Log("msg", "setting count frequency", "freq", cfg.Frequency)
	t.cfg = cfg
	return nil
}
