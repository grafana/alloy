// Package receiver utilities to create a Flow component from OpenTelemetry
// Collector receivers.
package receiver

import (
	"context"
	"errors"
	"os"

	"github.com/grafana/agent/internal/build"
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/agent/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/agent/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/agent/internal/component/otelcol/internal/views"
	"github.com/grafana/agent/internal/util/zapadapter"
	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	otelreceiver "go.opentelemetry.io/collector/receiver"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector receivers.
type Arguments interface {
	component.Arguments

	// Convert converts the Arguments into an OpenTelemetry Collector receiver
	// configuration.
	Convert() (otelcomponent.Config, error)

	// Extensions returns the set of extensions that the configured component is
	// allowed to use.
	Extensions() map[otelcomponent.ID]extension.Extension

	// Exporters returns the set of exporters that are exposed to the configured
	// component.
	Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component

	// NextConsumers returns the set of consumers to send data to.
	NextConsumers() *otelcol.ConsumerArguments

	// DebugMetricsConfig returns the configuration for debug metrics
	DebugMetricsConfig() otelcol.DebugMetricsArguments
}

// Receiver is a Flow component shim which manages an OpenTelemetry Collector
// receiver component.
type Receiver struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts    component.Options
	factory otelreceiver.Factory

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector
}

var (
	_ component.Component       = (*Receiver)(nil)
	_ component.HealthComponent = (*Receiver)(nil)
)

// New creates a new Flow component which encapsulates an OpenTelemetry
// Collector receiver. args must hold a value of the argument type registered
// with the Flow component.
//
// If the registered Flow component registers exported fields, it is the
// responsibility of the caller to export values when needed; the Receiver
// component never exports any values.
func New(opts component.Options, f otelreceiver.Factory, args Arguments) (*Receiver, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a lazy collector where metrics from the upstream component will be
	// forwarded.
	collector := lazycollector.New()
	opts.Registerer.MustRegister(collector)

	r := &Receiver{
		ctx:    ctx,
		cancel: cancel,

		opts:    opts,
		factory: f,

		sched:     scheduler.New(opts.Logger),
		collector: collector,
	}
	if err := r.Update(args); err != nil {
		return nil, err
	}
	return r, nil
}

// Run starts the Receiver component.
func (r *Receiver) Run(ctx context.Context) error {
	defer r.cancel()
	return r.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector receiver configuration and manage
// the underlying OpenTelemetry Collector receiver.
func (r *Receiver) Update(args component.Arguments) error {
	rargs := args.(Arguments)

	host := scheduler.NewHost(
		r.opts.Logger,
		scheduler.WithHostExtensions(rargs.Extensions()),
		scheduler.WithHostExporters(rargs.Exporters()),
	)

	reg := prometheus.NewRegistry()
	r.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	metricOpts := []metric.Option{metric.WithReader(promExporter)}
	if rargs.DebugMetricsConfig().DisableHighCardinalityMetrics {
		metricOpts = append(metricOpts, metric.WithView(views.DropHighCardinalityServerAttributes()...))
	}

	settings := otelreceiver.CreateSettings{
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(r.opts.Logger),

			TracerProvider: r.opts.Tracer,
			MeterProvider:  metric.NewMeterProvider(metricOpts...),

			ReportComponentStatus: func(*otelcomponent.StatusEvent) error {
				return nil
			},
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Agent",
			Version:     build.Version,
		},
	}

	receiverConfig, err := rargs.Convert()
	if err != nil {
		return err
	}

	var (
		next        = rargs.NextConsumers()
		nextTraces  = fanoutconsumer.Traces(next.Traces)
		nextMetrics = fanoutconsumer.Metrics(next.Metrics)
		nextLogs    = fanoutconsumer.Logs(next.Logs)
	)

	// Create instances of the receiver from our factory for each of our
	// supported telemetry signals.
	var components []otelcomponent.Component

	tracesReceiver, err := r.factory.CreateTracesReceiver(r.ctx, settings, receiverConfig, nextTraces)
	if err != nil && !errors.Is(err, otelcomponent.ErrDataTypeIsNotSupported) {
		return err
	} else if tracesReceiver != nil {
		components = append(components, tracesReceiver)
	}

	metricsReceiver, err := r.factory.CreateMetricsReceiver(r.ctx, settings, receiverConfig, nextMetrics)
	if err != nil && !errors.Is(err, otelcomponent.ErrDataTypeIsNotSupported) {
		return err
	} else if metricsReceiver != nil {
		components = append(components, metricsReceiver)
	}

	logsReceiver, err := r.factory.CreateLogsReceiver(r.ctx, settings, receiverConfig, nextLogs)
	if err != nil && !errors.Is(err, otelcomponent.ErrDataTypeIsNotSupported) {
		return err
	} else if logsReceiver != nil {
		components = append(components, logsReceiver)
	}

	// Schedule the components to run once our component is running.
	r.sched.Schedule(host, components...)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (r *Receiver) CurrentHealth() component.Health {
	return r.sched.CurrentHealth()
}
