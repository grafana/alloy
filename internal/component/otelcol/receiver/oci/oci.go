// Package oci provides an otelcol.receiver.oci component.
package oci

import (
	"context"
	"log/slog"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/oci-exporter/pkg/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.oci",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Component is the otelcol.receiver.oci component.
type Component struct {
	log  log.Logger
	opts component.Options

	mut     sync.Mutex
	cfg     Arguments
	exp     *exporter.Exporter
	metrics chan pmetric.Metrics

	debugDataPublisher livedebugging.DebugDataPublisher
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new otelcol.receiver.oci component.
func New(o component.Options, c Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	res := &Component{
		log:                o.Logger,
		opts:               o,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	if err := res.Update(c); err != nil {
		return nil, err
	}
	return res, nil
}

// Run starts the exporter background scraping and pushes metrics downstream
// on each scrape cycle via the OnScrape callback.
func (c *Component) Run(ctx context.Context) error {
	c.mut.Lock()
	exp := c.exp
	cfg := c.cfg
	metricsCh := c.metrics
	c.mut.Unlock()

	if exp == nil {
		<-ctx.Done()
		return nil
	}

	// Build the consumer chain for pushing metrics downstream.
	nextMetrics := cfg.Output.Metrics
	fanout := fanoutconsumer.Metrics(nextMetrics)
	metricsConsumer := interceptconsumer.Metrics(fanout,
		func(ctx context.Context, md pmetric.Metrics) error {
			livedebuggingpublisher.PublishMetricsIfActive(c.debugDataPublisher, c.opts.ID, md, otelcol.GetComponentMetadata(nextMetrics))
			return fanout.ConsumeMetrics(ctx, md)
		},
	)

	// Start the exporter's background discovery and scrape cycles.
	go func() {
		_ = exp.Start(ctx)
	}()

	// Receive metrics pushed by the OnScrape callback and forward downstream.
	for {
		select {
		case <-ctx.Done():
			return nil
		case md := <-metricsCh:
			if md.MetricCount() == 0 {
				continue
			}
			_ = metricsConsumer.ConsumeMetrics(ctx, md)
		}
	}
}

// Update implements Component. It rebuilds the exporter when config changes.
func (c *Component) Update(newConfig component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	cfg := newConfig.(Arguments)

	debug := cfg.Debug
	l := slog.New(newSlogHandler(logging.NewSlogGoKitHandler(c.log), debug))

	metricsCh := make(chan pmetric.Metrics, 1)
	onScrape := func(md pmetric.Metrics) {
		select {
		case metricsCh <- md:
		default:
			// Drop if the consumer isn't keeping up.
		}
	}

	ociCfg := cfg.Convert()
	exp, err := exporter.New(ociCfg, exporter.WithLogger(l), exporter.WithOnScrape(onScrape))
	if err != nil {
		return err
	}

	// Stop the old exporter if it was running.
	if c.exp != nil {
		c.exp.Stop()
	}

	c.cfg = cfg
	c.exp = exp
	c.metrics = metricsCh

	return nil
}

func (c *Component) LiveDebugging() {}
