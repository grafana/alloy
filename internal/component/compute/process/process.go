package process

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/prometheus/storage"
)

func init() {
	component.Register(component.Registration{
		Name:      "compute.process",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Component struct {
	mut  sync.RWMutex
	wasm *WasmPlugin
	loki loki.LogsReceiver
	args Arguments
	opts component.Options
}

func New(opts component.Options, args Arguments) (*Component, error) {
	wp, err := NewPlugin(args.Wasm, args.Config, context.TODO())
	if err != nil {
		return nil, err
	}

	c := &Component{
		wasm: wp,
		opts: opts,
		args: args,
	}
	c.opts.OnStateChange(Exports{
		PrometheusReceiver: c,
		LokiReceiver:       c.loki,
	})
	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if slices.Equal(c.args.Wasm, args.(Arguments).Wasm) && maps.Equal(c.args.Config, args.(Arguments).Config) {
		return nil
	}
	c.args = args.(Arguments)

	return nil
}

func (c *Component) Appender(ctx context.Context) storage.Appender {
	return &bulkAppender{
		ctx:  ctx,
		wasm: c.wasm,
		next: c.args.PrometheusForwardTo,
	}
}
