// Package exporter exposes utilities to create an Alloy component from
// OpenTelemetry Collector exporters.
package exporter

import (
	"context"
	"errors"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	otelexporter "go.opentelemetry.io/collector/exporter"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
	sdkprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazycollector"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/internal/scheduler"
	"github.com/grafana/alloy/internal/component/otelcol/internal/views"
	"github.com/grafana/alloy/internal/util/zapadapter"
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector exporters.
type Arguments interface {
	component.Arguments

	// Convert converts the Arguments into an OpenTelemetry Collector exporter
	// configuration.
	Convert() (otelcomponent.Config, error)

	// Extensions returns the set of extensions that the configured component is
	// allowed to use.
	Extensions() map[otelcomponent.ID]otelextension.Extension

	// Exporters returns the set of exporters that are exposed to the configured
	// component.
	Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component

	// DebugMetricsConfig returns the configuration for debug metrics
	DebugMetricsConfig() otelcolCfg.DebugMetricsArguments
}

// TypeSignal is a bit field to indicate which telemetry signals the exporter supports.
type TypeSignal byte

const (
	TypeLogs    TypeSignal = 1 << iota // 1
	TypeMetrics                        // 2
	TypeTraces                         // 4
)

// TypeAll indicates that the exporter supports all telemetry signals.
const TypeAll = TypeLogs | TypeMetrics | TypeTraces

// SupportsLogs returns true if the exporter supports logs.
func (s TypeSignal) SupportsLogs() bool {
	return s&TypeLogs != 0
}

// SupportsMetrics returns true if the exporter supports metrics.
func (s TypeSignal) SupportsMetrics() bool {
	return s&TypeMetrics != 0
}

// SupportsTraces returns true if the exporter supports traces.
func (s TypeSignal) SupportsTraces() bool {
	return s&TypeTraces != 0
}

// Exporter is an Alloy component shim which manages an OpenTelemetry Collector
// exporter component.
type Exporter struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts     component.Options
	factory  otelexporter.Factory
	consumer *lazyconsumer.Consumer

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector

	// Signals which the exporter is able to export.
	// Can be logs, metrics, traces or any combination of them.
	supportedSignals TypeSignal
}

var (
	_ component.Component       = (*Exporter)(nil)
	_ component.HealthComponent = (*Exporter)(nil)
)

// New creates a new component which encapsulates an OpenTelemetry Collector
// exporter. args must hold a value of the argument type registered with the
// Alloy component.
//
// The registered component must be registered to export the
// otelcol.ConsumerExports type, otherwise New will panic.
func New(opts component.Options, f otelexporter.Factory, args Arguments, supportedSignals TypeSignal) (*Exporter, error) {
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

	e := &Exporter{
		ctx:    ctx,
		cancel: cancel,

		opts:     opts,
		factory:  f,
		consumer: consumer,

		sched:     scheduler.NewWithPauseCallbacks(opts.Logger, consumer.Pause, consumer.Resume),
		collector: collector,

		supportedSignals: supportedSignals,
	}
	if err := e.Update(args); err != nil {
		return nil, err
	}
	return e, nil
}

// Run starts the Exporter component.
func (e *Exporter) Run(ctx context.Context) error {
	defer e.cancel()
	return e.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector exporter configuration and manage
// the underlying OpenTelemetry Collector exporter.
func (e *Exporter) Update(args component.Arguments) error {
	eargs := args.(Arguments)

	host := scheduler.NewHost(
		e.opts.Logger,
		scheduler.WithHostExtensions(eargs.Extensions()),
		scheduler.WithHostExporters(eargs.Exporters()),
	)

	reg := prometheus.NewRegistry()
	e.collector.Set(reg)

	promExporter, err := sdkprometheus.New(sdkprometheus.WithRegisterer(reg), sdkprometheus.WithoutTargetInfo())
	if err != nil {
		return err
	}

	debugMetricsConfig := eargs.DebugMetricsConfig()

	metricOpts := []metric.Option{metric.WithReader(promExporter)}
	if debugMetricsConfig.DisableHighCardinalityMetrics {
		metricOpts = append(metricOpts, metric.WithView(views.DropHighCardinalityServerAttributes()...))
	}

	metricsLevel, err := debugMetricsConfig.Level.Convert()
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metricOpts...)
	settings := otelexporter.Settings{
		TelemetrySettings: otelcomponent.TelemetrySettings{
			Logger: zapadapter.New(e.opts.Logger),

			TracerProvider: e.opts.Tracer,
			MeterProvider:  mp,
			LeveledMeterProvider: func(level configtelemetry.Level) otelmetric.MeterProvider {
				if level <= metricsLevel {
					return mp
				}
				return noop.MeterProvider{}
			},
			MetricsLevel: metricsLevel,
		},

		BuildInfo: otelcomponent.BuildInfo{
			Command:     os.Args[0],
			Description: "Grafana Alloy",
			Version:     build.Version,
		},
	}

	exporterConfig, err := eargs.Convert()
	if err != nil {
		return err
	}

	// Create instances of the exporter from our factory for each of our
	// supported telemetry signals.
	var components []otelcomponent.Component

	var tracesExporter otelexporter.Traces
	if e.supportedSignals.SupportsTraces() {
		tracesExporter, err = e.factory.CreateTracesExporter(e.ctx, settings, exporterConfig)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if tracesExporter != nil {
			components = append(components, tracesExporter)
		}
	}

	var metricsExporter otelexporter.Metrics
	if e.supportedSignals.SupportsMetrics() {
		metricsExporter, err = e.factory.CreateMetricsExporter(e.ctx, settings, exporterConfig)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if metricsExporter != nil {
			components = append(components, metricsExporter)
		}
	}

	var logsExporter otelexporter.Logs
	if e.supportedSignals.SupportsLogs() {
		logsExporter, err = e.factory.CreateLogsExporter(e.ctx, settings, exporterConfig)
		if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
			return err
		} else if logsExporter != nil {
			components = append(components, logsExporter)
		}
	}

	// Schedule the components to run once our component is running.
	e.sched.Schedule(host, components...)
	e.consumer.SetConsumers(tracesExporter, metricsExporter, logsExporter)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (e *Exporter) CurrentHealth() component.Health {
	return e.sched.CurrentHealth()
}
