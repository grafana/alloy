package process

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
	prom "github.com/prometheus/client_golang/prometheus"
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
	mut                        sync.RWMutex
	wasm                       *WasmPlugin
	loki                       loki.LogsReceiver
	args                       Arguments
	opts                       component.Options
	ls                         labelstore.LabelStore
	timeMetric                 prom.Counter
	prometheusRecordsProcessed prom.Counter
	bridge                     *bridge
}

func New(opts component.Options, args Arguments) (*Component, error) {
	data, err := opts.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	wp, err := NewPlugin(args.Wasm, args.Config, context.TODO())
	if err != nil {
		return nil, err
	}

	c := &Component{
		wasm: wp,
		opts: opts,
		args: args,
		ls:   data.(labelstore.LabelStore),
		timeMetric: prom.NewCounter(prom.CounterOpts{
			Namespace: "alloy",
			Subsystem: "compute",
			Name:      "process_time_ms_total",
		}),
		prometheusRecordsProcessed: prom.NewCounter(prom.CounterOpts{
			Namespace: "alloy",
			Subsystem: "compute",
			Name:      "process_prometheus_records_processed",
		}),
	}
	// TODO creation of the bridge, loki receiver and bulk appendable is all really really lackluster.
	br := &bridge{
		lr: &lokiReceiver{
			batchSize: 1_000,
			interval:  500 * time.Millisecond,
			wasm:      wp,
			channel:   make(chan loki.Entry),
			logs:      make([]loki.Entry, 0),
			forwardto: args.LokiForwardTo,
			stop:      make(chan struct{}),
		},
	}
	ba := &bulkAppendable{
		wasm:                       wp,
		metrics:                    make([]*PrometheusMetric, 0),
		next:                       prometheus.NewFanout(c.args.PrometheusForwardTo, c.opts.ID, c.opts.Registerer, c.ls),
		timeMetric:                 c.timeMetric,
		prometheusRecordsProcessed: c.prometheusRecordsProcessed,
		bridge:                     br,
	}
	br.prom = ba
	c.bridge = br
	c.opts.Registerer.Register(c.timeMetric)
	c.opts.Registerer.Register(c.prometheusRecordsProcessed)
	c.opts.OnStateChange(Exports{
		PrometheusReceiver: ba,
		LokiReceiver:       c.loki,
	})
	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	// Start the loki receiver on the bridge.
	// TODO should likely make this a start on the bridge itself.
	c.bridge.lr.Start(ctx)
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
