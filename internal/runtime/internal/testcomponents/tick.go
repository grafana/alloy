package testcomponents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.tick",
		Stability: featuregate.StabilityPublicPreview,
		Args:      TickConfig{},
		Exports:   TickExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewTick(opts, args.(TickConfig))
		},
	})
}

// TickConfig configures the testcomponents.tick component.
type TickConfig struct {
	Frequency time.Duration `alloy:"frequency,attr"`
}

// TickExports describes exported fields for the testcomponents.tick component.
type TickExports struct {
	Time time.Time `alloy:"tick_time,attr,optional"`
}

// Tick implements the testcomponents.tick component, where the wallclock time
// will be emitted on a given frequency.
type Tick struct {
	opts component.Options
	log  log.Logger

	cfgMut sync.Mutex
	cfg    TickConfig
}

// NewTick creates a new testcomponents.tick component.
func NewTick(o component.Options, cfg TickConfig) (*Tick, error) {
	t := &Tick{opts: o, log: o.Logger}
	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Tick)(nil)
)

// Run implements Component.
func (t *Tick) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(t.getNextTick()):
			level.Info(t.log).Log("msg", "ticked")
			t.opts.OnStateChange(TickExports{Time: time.Now()})
		}
	}
}

func (t *Tick) getNextTick() time.Duration {
	t.cfgMut.Lock()
	defer t.cfgMut.Unlock()
	return t.cfg.Frequency
}

// Update implements Component.
func (t *Tick) Update(args component.Arguments) error {
	t.cfgMut.Lock()
	defer t.cfgMut.Unlock()

	cfg := args.(TickConfig)
	if cfg.Frequency == 0 {
		return fmt.Errorf("frequency must not be 0")
	}

	level.Info(t.log).Log("msg", "setting tick frequency", "freq", cfg.Frequency)
	t.cfg = cfg
	return nil
}
