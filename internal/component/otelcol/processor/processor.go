// Package processor exposes utilities to create an Alloy component from
// OpenTelemetry Collector processors.
package processor

import (
	"context"
	"errors"
	"os"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fanoutconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/livedebuggingconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util/zapadapter"
	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	otelprocessor "go.opentelemetry.io/collector/processor"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
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
	Extensions() map[otelcomponent.ID]otelextension.Extension

	// Exporters returns the set of exporters that are exposed to the configured
	// component.
	Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component

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

	liveDebuggingConsumer *livedebuggingconsumer.Consumer
	debugDataPublisher    livedebugging.DebugDataPublisher

	args Arguments
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

	consumer := lazyconsumer.New(ctx)

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

		sched:     scheduler.New(opts.Logger),
		collector: collector,

		liveDebuggingConsumer: livedebuggingconsumer.New(debugDataPublisher.(livedebugging.DebugDataPublisher), opts.ID),
		debugDataPublisher:    debugDataPublisher.(livedebugging.DebugDataPublisher),
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

	metricsLevel, err := p.args.DebugMetricsConfig().Level.Convert()
	if err != nil {
		return err
	}

	settings := otelprocessor.CreateSettings{
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(p.opts.Logger),

			TracerProvider: p.opts.Tracer,
			MeterProvider:  metric.NewMeterProvider(metric.WithReader(promExporter)),
			MetricsLevel:   metricsLevel,

			ReportStatus: func(*otelcomponent.StatusEvent) {},
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Alloy",
			Version:     build.Version,
		},
	}

	processorConfig, err := p.args.Convert()
	if err != nil {
		return err
	}

	next := p.args.NextConsumers()
	traces, metrics, logs := next.Traces, next.Metrics, next.Logs

	if p.debugDataPublisher.IsActive(livedebugging.ComponentID(p.opts.ID)) {
		traces = append(traces, p.liveDebuggingConsumer)
		metrics = append(metrics, p.liveDebuggingConsumer)
		logs = append(logs, p.liveDebuggingConsumer)
	}

	var (
		nextTraces  = fanoutconsumer.Traces(traces)
		nextMetrics = fanoutconsumer.Metrics(metrics)
		nextLogs    = fanoutconsumer.Logs(logs)
	)

	// Create instances of the processor from our factory for each of our
	// supported telemetry signals.
	var components []otelcomponent.Component

	var tracesProcessor otelprocessor.Traces
	if len(next.Traces) > 0 {
		tracesProcessor, err = p.factory.CreateTracesProcessor(p.ctx, settings, processorConfig, nextTraces)
		if err != nil && !errors.Is(err, otelcomponent.ErrDataTypeIsNotSupported) {
			return err
		} else if tracesProcessor != nil {
			components = append(components, tracesProcessor)
		}
	}

	var metricsProcessor otelprocessor.Metrics
	if len(next.Metrics) > 0 {
		metricsProcessor, err = p.factory.CreateMetricsProcessor(p.ctx, settings, processorConfig, nextMetrics)
		if err != nil && !errors.Is(err, otelcomponent.ErrDataTypeIsNotSupported) {
			return err
		} else if metricsProcessor != nil {
			components = append(components, metricsProcessor)
		}
	}

	var logsProcessor otelprocessor.Logs
	if len(next.Logs) > 0 {
		logsProcessor, err = p.factory.CreateLogsProcessor(p.ctx, settings, processorConfig, nextLogs)
		if err != nil && !errors.Is(err, otelcomponent.ErrDataTypeIsNotSupported) {
			return err
		} else if logsProcessor != nil {
			components = append(components, logsProcessor)
		}
	}

	// Schedule the components to run once our component is running.
	p.sched.Schedule(host, components...)
	p.consumer.SetConsumers(tracesProcessor, metricsProcessor, logsProcessor)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (p *Processor) CurrentHealth() component.Health {
	return p.sched.CurrentHealth()
}

func (p *Processor) LiveDebugging(_ int) {
	p.Update(p.args)
}
