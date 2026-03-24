package testcomponent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.ticker",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      TickerConfig{},
		Exports:   TickerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewTicker(opts, args.(TickerConfig))
		},
	})
}

type TickerConfig struct {
	Period   time.Duration `alloy:"period,attr"`
	MaxValue int           `alloy:"max_value,attr"`
}

type TickerExports struct {
	Counter int `alloy:"counter,attr"`
}

type Ticker struct {
	opts component.Options

	mutex          sync.Mutex
	args           TickerConfig
	currentCounter int

	counterGauge prometheus.Gauge
}

func NewTicker(o component.Options, cfg TickerConfig) (*Ticker, error) {
	t := &Ticker{
		opts: o,
		counterGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ticker_counter",
			Help: "The current value of the ticker counter",
		}),
	}

	if err := o.Registerer.Register(t.counterGauge); err != nil {
		return nil, fmt.Errorf("failed to register counter gauge: %w", err)
	}

	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

var (
	_ component.Component = (*Ticker)(nil)
)

func (t *Ticker) Run(ctx context.Context) error {
	var period time.Duration
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(period):
			t.mutex.Lock()
			args := t.args
			period = t.args.Period
			if t.currentCounter < args.MaxValue {
				t.currentCounter += 1
			}
			counter := t.currentCounter
			t.mutex.Unlock()

			level.Info(t.opts.Logger).Log("msg", "tick", "counter", counter)
			t.opts.OnStateChange(TickerExports{Counter: counter})
			t.counterGauge.Set(float64(counter))
		}
	}
}

func (t *Ticker) Update(args component.Arguments) error {
	cfg := args.(TickerConfig)
	if cfg.Period <= 0 {
		return fmt.Errorf("period must be greater than 0")
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.args = cfg

	level.Info(t.opts.Logger).Log("msg", "updated ticker", "cfg", cfg)
	return nil
}
