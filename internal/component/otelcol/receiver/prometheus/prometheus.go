// Package prometheus provides an otelcol.receiver.prometheus component.
package prometheus

import (
	"context"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/prometheus/internal"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelreceiver "go.opentelemetry.io/collector/receiver"
	metricNoop "go.opentelemetry.io/otel/metric/noop"
	traceNoop "go.opentelemetry.io/otel/trace/noop"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.receiver.prometheus",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(o component.Options, a component.Arguments) (component.Component, error) {
			return New(o, a.(Arguments))
		},
	})
}

// Arguments configures the otelcol.receiver.prometheus component.
type Arguments struct {
	// Output configures where to send received data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`
	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

// Exports are the set of fields exposed by the otelcol.receiver.prometheus
// component.
type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// Component is the otelcol.receiver.prometheus component.
type Component struct {
	log  log.Logger
	opts component.Options

	mut        sync.RWMutex
	cfg        Arguments
	appendable storage.Appendable

	liveDebuggingConsumer *livedebuggingconsumer.Consumer
	debugDataPublisher    livedebugging.DebugDataPublisher
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new otelcol.receiver.prometheus component.
func New(o component.Options, c Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	res := &Component{
		log:                   o.Logger,
		opts:                  o,
		liveDebuggingConsumer: livedebuggingconsumer.New(debugDataPublisher.(livedebugging.DebugDataPublisher), o.ID),
		debugDataPublisher:    debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	if err := res.Update(c); err != nil {
		return nil, err
	}
	return res, nil
}

// Run implements Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements Component.
func (c *Component) Update(newConfig component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	cfg := newConfig.(Arguments)
	c.cfg = cfg

	// useStartTimeMetric is used to configure the 'metrics adjuster' in the
	// prometheusreceiver which is applied whenever a Commit is called.
	// If set to true, the receiver will utilize a `startTimeMetricAdjuster`
	// to adjust metric start times based on a start time metric. The start
	// time metric defaults to `process_start_time_seconds`, but can be
	// overridden by using this regex.
	//
	// gcInterval should be at least as long as the longest scrape interval
	// used by the upstream scrape configs, plus a delta to avoid race
	// conditions so that jobs are not getting GC'ed between scrapes.
	var (
		useStartTimeMetric   = false
		startTimeMetricRegex *regexp.Regexp

		// Start time for Summary, Histogram and Sum metrics can be retrieved from `_created` metrics.
		useCreatedMetric = false

		// Trimming the metric suffixes is used to remove the metric type and the unit and the end of the metric name.
		// To trim the unit, the opentelemetry code uses the MetricMetadataStore which is currently not supported by Alloy.
		// When supported, this could be added as an arg.
		trimMetricSuffixes = false

		enableNativeHistograms = c.opts.MinStability.Permits(featuregate.StabilityPublicPreview)

		gcInterval = 5 * time.Minute
	)

	metricsLevel, err := cfg.DebugMetrics.Level.Convert()
	if err != nil {
		return err
	}

	settings := otelreceiver.CreateSettings{
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(c.opts.Logger),

			// TODO(tpaschalis): expose tracing and logging statistics.
			TracerProvider: traceNoop.NewTracerProvider(),
			MeterProvider:  metricNoop.NewMeterProvider(),
			MetricsLevel:   metricsLevel,

			ReportStatus: func(*otelcomponent.StatusEvent) {},
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Alloy",
			Version:     build.Version,
		},
	}
	metrics := cfg.Output.Metrics
	if c.debugDataPublisher.IsActive(livedebugging.ComponentID(c.opts.ID)) {
		metrics = append(metrics, c.liveDebuggingConsumer)
	}
	metricsSink := fanoutconsumer.Metrics(metrics)

	appendable, err := internal.NewAppendable(
		metricsSink,
		settings,
		gcInterval,
		useStartTimeMetric,
		startTimeMetricRegex,
		useCreatedMetric,
		enableNativeHistograms,
		labels.Labels{},
		trimMetricSuffixes,
	)
	if err != nil {
		return err
	}
	c.appendable = appendable

	// Export the receiver.
	c.opts.OnStateChange(Exports{Receiver: c.appendable})

	return nil
}

func (c *Component) LiveDebugging(_ int) {
	c.Update(c.cfg)
}
