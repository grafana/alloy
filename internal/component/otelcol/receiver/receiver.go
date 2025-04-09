// Package receiver utilities to create an Alloy component from OpenTelemetry
// Collector receivers.
package receiver

import (
	"context"
	"errors"
	"os"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/component/otelcol/internal/views"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
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
	Extensions() map[otelcomponent.ID]otelcomponent.Component

	// Exporters returns the set of exporters that are exposed to the configured
	// component.
	Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component

	// NextConsumers returns the set of consumers to send data to.
	NextConsumers() *otelcol.ConsumerArguments

	// DebugMetricsConfig returns the configuration for debug metrics
	DebugMetricsConfig() otelcolCfg.DebugMetricsArguments
}

// Receiver is an Alloy component shim which manages an OpenTelemetry Collector
// receiver component.
type Receiver struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts    component.Options
	factory otelreceiver.Factory

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector

	debugDataPublisher livedebugging.DebugDataPublisher

	args Arguments
}

var (
	_ component.Component       = (*Receiver)(nil)
	_ component.HealthComponent = (*Receiver)(nil)
	_ component.LiveDebugging   = (*Receiver)(nil)
)

// New creates a new Alloy component which encapsulates an OpenTelemetry
// Collector receiver. args must hold a value of the argument type registered
// with the Alloy component.
//
// If the registered Alloy component registers exported fields, it is the
// responsibility of the caller to export values when needed; the Receiver
// component never exports any values.
func New(opts component.Options, f otelreceiver.Factory, args Arguments) (*Receiver, error) {
	debugDataPublisher, err := opts.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

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

		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
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
	r.args = args.(Arguments)
	host := scheduler.NewHost(
		r.opts.Logger,
		scheduler.WithHostExtensions(r.args.Extensions()),
		scheduler.WithHostExporters(r.args.Exporters()),
	)

	reg := prometheus.NewRegistry()
	r.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	debugMetricsCfg := r.args.DebugMetricsConfig()
	metricOpts := []metric.Option{metric.WithReader(promExporter)}
	if debugMetricsCfg.DisableHighCardinalityMetrics {
		metricOpts = append(metricOpts, metric.WithView(views.DropHighCardinalityServerAttributes()...))
	}

	mp := metric.NewMeterProvider(metricOpts...)
	settings := otelreceiver.Settings{
		ID: otelcomponent.NewIDWithName(r.factory.Type(), r.opts.ID),
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(r.opts.Logger),

			TracerProvider: r.opts.Tracer,
			MeterProvider:  mp,
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Alloy",
			Version:     build.Version,
		},
	}

	receiverConfig, err := r.args.Convert()
	if err != nil {
		return err
	}

	next := r.args.NextConsumers()

	// Create instances of the receiver from our factory for each of our
	// supported telemetry signals.
	var components []otelcomponent.Component

	if len(next.Traces) > 0 {
		fanout := fanoutconsumer.Traces(next.Traces)
		tracesInterceptor := interceptconsumer.Traces(fanout,
			func(ctx context.Context, td ptrace.Traces) error {
				livedebuggingpublisher.PublishTracesIfActive(r.debugDataPublisher, r.opts.ID, td, otelcol.GetComponentMetadata(next.Traces))
				return fanout.ConsumeTraces(ctx, td)
			},
		)
		tracesReceiver, err := r.factory.CreateTraces(r.ctx, settings, receiverConfig, tracesInterceptor)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if tracesReceiver != nil {
			components = append(components, tracesReceiver)
		}
	}

	if len(next.Metrics) > 0 {
		fanout := fanoutconsumer.Metrics(next.Metrics)
		metricsInterceptor := interceptconsumer.Metrics(fanout,
			func(ctx context.Context, md pmetric.Metrics) error {
				livedebuggingpublisher.PublishMetricsIfActive(r.debugDataPublisher, r.opts.ID, md, otelcol.GetComponentMetadata(next.Metrics))
				return fanout.ConsumeMetrics(ctx, md)
			},
		)
		metricsReceiver, err := r.factory.CreateMetrics(r.ctx, settings, receiverConfig, metricsInterceptor)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if metricsReceiver != nil {
			components = append(components, metricsReceiver)
		}
	}

	if len(next.Logs) > 0 {
		fanout := fanoutconsumer.Logs(next.Logs)
		logsInterceptor := interceptconsumer.Logs(fanout,
			func(ctx context.Context, ld plog.Logs) error {
				livedebuggingpublisher.PublishLogsIfActive(r.debugDataPublisher, r.opts.ID, ld, otelcol.GetComponentMetadata(next.Logs))
				return fanout.ConsumeLogs(ctx, ld)
			},
		)
		logsReceiver, err := r.factory.CreateLogs(r.ctx, settings, receiverConfig, logsInterceptor)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if logsReceiver != nil {
			components = append(components, logsReceiver)
		}
	}

	// Schedule the components to run once our component is running.
	r.sched.Schedule(r.ctx, func() {}, host, components...)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (r *Receiver) CurrentHealth() component.Health {
	return r.sched.CurrentHealth()
}

func (p *Receiver) LiveDebugging() {}
