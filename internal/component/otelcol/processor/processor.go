// Package processor exposes utilities to create an Alloy component from
// OpenTelemetry Collector processors.
package processor

import (
	"context"
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	otelprocessor "go.opentelemetry.io/collector/processor"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/interceptconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingpublisher"
	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	otelcolutil "github.com/grafana/alloy/internal/component/otelcol/util"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util/zapadapter"
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector processors.
type Arguments interface {
	component.Arguments

	// Convert converts the Arguments into an OpenTelemetry Collector processor
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

// Processor is an Alloy component shim which manages an OpenTelemetry
// Collector processor component.
type Processor struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts     component.Options
	factory  otelprocessor.Factory
	consumer *lazyconsumer.Consumer

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector

	debugDataPublisher livedebugging.DebugDataPublisher

	args Arguments

	updateMut sync.Mutex
}

var (
	_ component.Component       = (*Processor)(nil)
	_ component.HealthComponent = (*Processor)(nil)
	_ component.LiveDebugging   = (*Processor)(nil)
)

// New creates a new Alloy component which encapsulates an OpenTelemetry
// Collector processor. args must hold a value of the argument type registered
// with the Alloy component.
//
// The registered component must be registered to export the
// otelcol.ConsumerExports type, otherwise New will panic.
func New(opts component.Options, f otelprocessor.Factory, args Arguments) (*Processor, error) {
	debugDataPublisher, err := opts.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	consumer := lazyconsumer.NewPaused(ctx, opts.ID)

	// Create a lazy collector where metrics from the upstream component will be
	// forwarded.
	collector := lazycollector.New()
	opts.Registerer.MustRegister(collector)

	// Immediately set our state with our consumer. The exports will never change
	// throughout the lifetime of our component.
	//
	// This will panic if the wrapping component is not registered to export
	// otelcol.ConsumerExports.
	opts.OnStateChange(otelcol.ConsumerExports{Input: consumer})

	p := &Processor{
		ctx:    ctx,
		cancel: cancel,

		opts:     opts,
		factory:  f,
		consumer: consumer,

		sched:     scheduler.NewWithPauseCallbacks(opts.Logger, consumer.Pause, consumer.Resume),
		collector: collector,

		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}
	if err := p.Update(args); err != nil {
		return nil, err
	}
	return p, nil
}

// Run starts the Processor component.
func (p *Processor) Run(ctx context.Context) error {
	defer p.cancel()
	return p.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector processor configuration and manage
// the underlying OpenTelemetry Collector processor.
func (p *Processor) Update(args component.Arguments) error {
	p.updateMut.Lock()
	defer p.updateMut.Unlock()
	p.args = args.(Arguments)

	host := scheduler.NewHost(
		p.opts.Logger,
		scheduler.WithHostExtensions(p.args.Extensions()),
		scheduler.WithHostExporters(p.args.Exporters()),
	)

	reg := prometheus.NewRegistry()
	p.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metric.WithReader(promExporter))
	settings := otelprocessor.Settings{
		ID: otelcomponent.NewIDWithName(p.factory.Type(), p.opts.ID),
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger:         zapadapter.New(p.opts.Logger),
			TracerProvider: p.opts.Tracer,
			MeterProvider:  mp,
		},

		BuildInfo: otelcolutil.GetBuildInfo(),
	}

	resource, err := otelcolutil.GetTelemetrySettingsResource()
	if err != nil {
		return err
	}
	settings.TelemetrySettings.Resource = resource

	processorConfig, err := p.args.Convert()
	if err != nil {
		return err
	}

	next := p.args.NextConsumers()

	// Create instances of the processor from our factory for each of our
	// supported telemetry signals.
	var components []otelcomponent.Component

	var tracesProcessor otelprocessor.Traces
	if len(next.Traces) > 0 {
		fanout := fanoutconsumer.Traces(next.Traces)
		tracesInterceptor := interceptconsumer.Traces(fanout,
			func(ctx context.Context, td ptrace.Traces) error {
				livedebuggingpublisher.PublishTracesIfActive(p.debugDataPublisher, p.opts.ID, td, otelcol.GetComponentMetadata(next.Traces))
				return fanout.ConsumeTraces(ctx, td)
			},
		)
		tracesProcessor, err = p.factory.CreateTraces(p.ctx, settings, processorConfig, tracesInterceptor)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if tracesProcessor != nil {
			components = append(components, tracesProcessor)
		}
	}

	var metricsProcessor otelprocessor.Metrics
	if len(next.Metrics) > 0 {
		fanout := fanoutconsumer.Metrics(next.Metrics)
		metricsInterceptor := interceptconsumer.Metrics(fanout,
			func(ctx context.Context, md pmetric.Metrics) error {
				livedebuggingpublisher.PublishMetricsIfActive(p.debugDataPublisher, p.opts.ID, md, otelcol.GetComponentMetadata(next.Metrics))
				return fanout.ConsumeMetrics(ctx, md)
			},
		)
		metricsProcessor, err = p.factory.CreateMetrics(p.ctx, settings, processorConfig, metricsInterceptor)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if metricsProcessor != nil {
			components = append(components, metricsProcessor)
		}
	}

	var logsProcessor otelprocessor.Logs
	if len(next.Logs) > 0 {
		fanout := fanoutconsumer.Logs(next.Logs)
		logsInterceptor := interceptconsumer.Logs(fanout,
			func(ctx context.Context, ld plog.Logs) error {
				livedebuggingpublisher.PublishLogsIfActive(p.debugDataPublisher, p.opts.ID, ld, otelcol.GetComponentMetadata(next.Logs))
				return fanout.ConsumeLogs(ctx, ld)
			},
		)
		logsProcessor, err = p.factory.CreateLogs(p.ctx, settings, processorConfig, logsInterceptor)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if logsProcessor != nil {
			components = append(components, logsProcessor)
		}
	}

	updateConsumersFunc := func() {
		p.consumer.SetConsumers(tracesProcessor, metricsProcessor, logsProcessor)
	}

	// Schedule the components to run once our component is running.
	p.sched.Schedule(p.ctx, updateConsumersFunc, host, components...)

	return nil
}

// CurrentHealth implements component.HealthComponent.
func (p *Processor) CurrentHealth() component.Health {
	return p.sched.CurrentHealth()
}

func (p *Processor) LiveDebugging() {}
