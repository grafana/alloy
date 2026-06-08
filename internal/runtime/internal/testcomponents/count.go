package testcomponents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.count",
		Stability: featuregate.StabilityPublicPreview,
		Args:      CountConfig{},
		Exports:   CountExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewCount(opts, args.(CountConfig))
		},
	})
}

type CountConfig struct {
	Frequency time.Duration `alloy:"frequency,attr"`
	Max       int           `alloy:"max,attr"`
}

type CountExports struct {
	Count int `alloy:"count,attr,optional"`
}

type Count struct {
	opts  component.Options
	count atomic.Int32

	cfgMut sync.Mutex
	cfg    CountConfig
}

func NewCount(o component.Options, cfg CountConfig) (*Count, error) {
	t := &Count{opts: o}
	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Count)(nil)
)

func (t *Count) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(t.getNextCount()):
			t.cfgMut.Lock()
			maxCount := t.cfg.Max
			t.cfgMut.Unlock()

			currentCount := t.count.Load()
			if maxCount == 0 || currentCount < int32(maxCount) {
				if t.count.CompareAndSwap(currentCount, currentCount+1) {
					t.opts.SLogger.Info("incremented count", "count", currentCount+1)
					t.opts.OnStateChange(CountExports{Count: int(currentCount + 1)})
				} else {
					t.opts.SLogger.Error("failed to increment count", "count", currentCount)
				}
			}
		}
	}
}

func (t *Count) getNextCount() time.Duration {
	t.cfgMut.Lock()
	defer t.cfgMut.Unlock()
	return t.cfg.Frequency
}

// Update implements Component.
func (t *Count) Update(args component.Arguments) error {
	t.cfgMut.Lock()
	defer t.cfgMut.Unlock()

	cfg := args.(CountConfig)
	if cfg.Frequency == 0 {
		return fmt.Errorf("frequency must not be 0")
	}

	t.opts.SLogger.Info("setting count frequency", "freq", cfg.Frequency)
	t.cfg = cfg
	return nil
}
