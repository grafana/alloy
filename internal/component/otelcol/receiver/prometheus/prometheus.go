// Package prometheus provides an otelcol.receiver.prometheus component.
package prometheus

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/prometheus/internal"
	otelcolutil "github.com/grafana/alloy/internal/component/otelcol/util"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

	debugDataPublisher livedebugging.DebugDataPublisher
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
		log:                o.Logger,
		opts:               o,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
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

	var (
		// Trimming the metric suffixes is used to remove the metric type and the unit and the end of the metric name.
		// To trim the unit, the opentelemetry code uses the MetricMetadataStore which is currently not supported by Alloy.
		// When supported, this could be added as an arg.
		trimMetricSuffixes = false
	)

	mp := metricNoop.NewMeterProvider()
	settings := otelreceiver.Settings{
		ID: otelcomponent.NewIDWithName(otelcomponent.MustNewType("prometheus"), c.opts.ID),
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(c.opts.Logger),
			// TODO(tpaschalis): expose tracing and logging statistics.
			TracerProvider: traceNoop.NewTracerProvider(),
			MeterProvider:  mp,
		},

		BuildInfo: otelcolutil.GetBuildInfo(),
	}

	resource, err := otelcolutil.GetTelemetrySettingsResource()
	if err != nil {
		return err
	}
	settings.TelemetrySettings.Resource = resource

	nextMetrics := cfg.Output.Metrics
	fanout := fanoutconsumer.Metrics(nextMetrics)
	metricsInterceptor := interceptconsumer.Metrics(fanout,
		func(ctx context.Context, md pmetric.Metrics) error {
			livedebuggingpublisher.PublishMetricsIfActive(c.debugDataPublisher, c.opts.ID, md, otelcol.GetComponentMetadata(nextMetrics))
			return fanout.ConsumeMetrics(ctx, md)
		},
	)
	metricsSink := metricsInterceptor

	appendable, err := internal.NewAppendable(
		metricsSink,
		settings,
		true, // use metadata
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

func (c *Component) LiveDebugging() {}
