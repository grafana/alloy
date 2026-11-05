package rustgen

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/lokipipelinev2"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.rustgen",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.rustgen
// component.
type Arguments struct {
	ForwardTo     []loki.LogsReceiver `alloy:"forward_to,attr"`
	LinesPerBatch int                 `alloy:"lines_per_batch,attr,optional"`
	Interval      time.Duration       `alloy:"interval,attr,optional"`
	Labels        map[string]string   `alloy:"labels,attr,optional"`
}

var DefaultArguments = Arguments{
	LinesPerBatch: 100,
	Interval:      time.Second,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.LinesPerBatch <= 0 {
		return errors.New("lines_per_batch must be greater than 0")
	}
	if a.Interval < time.Millisecond {
		return errors.New("interval must be at least 1ms")
	}
	return nil
}

var _ component.Component = (*Component)(nil)

type Component struct {
	opts    component.Options
	fanout  *loki.FanoutV2
	restart chan struct{}

	mut  sync.Mutex
	args Arguments
}

// New creates a new loki.source.rustgen component.
func New(o component.Options, args Arguments) (*Component, error) {
	svc, err := o.GetServiceData(lokipipelinev2.ServiceName)
	if err != nil {
		return nil, err
	}

	if !svc.(lokipipelinev2.Data).PipelineV2Enabled() {
		return nil, errors.New("loki.source.rustgen requires Loki pipeline V2")
	}

	return &Component{
		opts:    o,
		fanout:  loki.NewFanoutV2(args.ForwardTo),
		restart: make(chan struct{}, 1),
		args:    args,
	}, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	for {
		c.mut.Lock()
		args := c.args
		c.mut.Unlock()

		gen, err := newGenerator(args.LinesPerBatch, args.Interval)
		if err != nil {
			return err
		}
		genCtx, cancel := context.WithCancel(ctx)

		var wg sync.WaitGroup
		wg.Go(gen.run)
		wg.Go(func() { c.generate(genCtx, gen, args) })

		select {
		case <-ctx.Done():
		case <-c.restart:
		}

		cancel()
		gen.close()
		wg.Wait()
		gen.free()

		if ctx.Err() != nil {
			return nil
		}
	}
}

func (c *Component) generate(ctx context.Context, gen *generator, args Arguments) {
	labels := make(model.LabelSet, len(args.Labels))
	for k, v := range args.Labels {
		labels[model.LabelName(k)] = model.LabelValue(v)
	}

	batcher := loki.NewStreamBatcherV2(c.opts.Logger, c.fanout, labels, args.LinesPerBatch)
	// Keep receiving after ctx is canceled, the Rust side can only stop once
	// everything it sent has been received.
	for gen.next(func(e push.Entry) { batcher.Add(e) }) {
		if err := batcher.Flush(ctx); err != nil && ctx.Err() == nil {
			c.opts.Logger.Error("failed to send entries", "err", err)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.mut.Lock()
	changed := newArgs.LinesPerBatch != c.args.LinesPerBatch ||
		newArgs.Interval != c.args.Interval ||
		!maps.Equal(newArgs.Labels, c.args.Labels)
	c.args = newArgs
	c.mut.Unlock()

	if changed {
		select {
		case c.restart <- struct{}{}:
		default:
		}
	}
	return nil
}
