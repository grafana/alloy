package process

import (
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/prometheus/prometheus/model/labels"
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

func (c *Component) Appender(ctx context.Context) prometheus.BulkAppender {
	return &bulkAppender{
		ctx:  ctx,
		wasm: c.wasm,
		next: c.args.PrometheusForwardTo,
	}
}

type bulkAppender struct {
	ctx  context.Context
	wasm *WasmPlugin
	next storage.Appender
}

func (b *bulkAppender) Append(metadata map[string]string, metrics []prometheus.PromMetric) error {
	pt := &Passthrough{
		// TODO reduce the number of random objects that
		// represent the same thing.
		Prommetrics: make([]*PrometheusMetric, len(metrics)),
	}
	for i, m := range metrics {
		labels := make([]*Label, len(m.Labels))
		for j, l := range m.Labels {
			labels[j] = &Label{
				Name:  l.Name,
				Value: l.Value,
			}
		}
		pt.Prommetrics[i] = &PrometheusMetric{
			Value:       m.Value,
			Timestampms: m.TS,
			Labels:      labels,
		}
	}
	outpt, err := b.wasm.Process(pt)
	if err != nil {
		return err
	}
	for _, m := range outpt.Prommetrics {
		labelsBack := make(labels.Labels, len(m.Labels))
		for i, l := range m.Labels {
			labelsBack[i] = labels.Label{
				Name:  l.Name,
				Value: l.Value,
			}
		}
		// We explicitly dont care about errors from append
		_, _ = b.next.Append(0, labelsBack, m.Timestampms, m.Value)
	}
	// We explicitly dont care about errors from commit
	_ = b.next.Commit()
	return nil

}
