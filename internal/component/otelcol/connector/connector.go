// Package connector exposes utilities to create an Alloy component from
// OpenTelemetry Collector connectors.
package connector

import (
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconnector "go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pipeline"
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

const (
	ConnectorTracesToTraces = iota
	ConnectorTracesToMetrics
	ConnectorTracesToLogs
	ConnectorMetricsToTraces
	ConnectorMetricsToMetrics
	ConnectorMetricsToLogs
	ConnectorLogsToTraces
	ConnectorLogsToMetrics
	ConnectorLogsToLogs
)

// Arguments is an extension of component.Arguments which contains necessary
// settings for OpenTelemetry Collector connectors.
type Arguments interface {
	component.Arguments

	// Convert converts the Arguments into an OpenTelemetry Collector connector
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

	ConnectorType() int

	// DebugMetricsConfig returns the configuration for debug metrics
	DebugMetricsConfig() otelcolCfg.DebugMetricsArguments
}

// Connector is an Alloy component shim which manages an OpenTelemetry
// Collector connector component.
type Connector struct {
	ctx    context.Context
	cancel context.CancelFunc

	opts     component.Options
	factory  otelconnector.Factory
	consumer *lazyconsumer.Consumer

	sched     *scheduler.Scheduler
	collector *lazycollector.Collector

	debugDataPublisher livedebugging.DebugDataPublisher

	args Arguments
}

var (
	_ component.Component       = (*Connector)(nil)
	_ component.HealthComponent = (*Connector)(nil)
	_ component.LiveDebugging   = (*Connector)(nil)
)

// New creates a new Alloy component which encapsulates an OpenTelemetry
// Collector connector. args must hold a value of the argument type registered
// with the Alloy component.
//
// The registered component must be registered to export the
// otelcol.ConsumerExports type, otherwise New will panic.
func New(opts component.Options, f otelconnector.Factory, args Arguments) (*Connector, error) {
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

	p := &Connector{
		ctx:    ctx,
		cancel: cancel,

		opts:     opts,
		factory:  f,
		consumer: consumer,

		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
		sched:              scheduler.NewWithPauseCallbacks(opts.Logger, consumer.Pause, consumer.Resume),
		collector:          collector,
	}
	if err := p.Update(args); err != nil {
		return nil, err
	}
	return p, nil
}

// Run starts the Connector component.
func (p *Connector) Run(ctx context.Context) error {
	defer p.cancel()
	return p.sched.Run(ctx)
}

// Update implements component.Component. It will convert the Arguments into
// configuration for OpenTelemetry Collector connector configuration and manage
// the underlying OpenTelemetry Collector connector.
func (p *Connector) Update(args component.Arguments) error {
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
	settings := otelconnector.Settings{
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

	connectorConfig, err := p.args.Convert()
	if err != nil {
		return err
	}

	next := p.args.NextConsumers()

	// Create instances of the connector from our factory for each of our
	// supported telemetry signals.
	var components []otelcomponent.Component

	var tracesConnector otelconnector.Traces
	var metricsConnector otelconnector.Metrics
	var logsConnector otelconnector.Logs

	connectorType := p.args.ConnectorType()

	// Check if connector outputs to metrics
	outputsToMetrics := (connectorType&ConnectorTracesToMetrics) != 0 ||
		(connectorType&ConnectorMetricsToMetrics) != 0 ||
		(connectorType&ConnectorLogsToMetrics) != 0

	if outputsToMetrics && (len(next.Traces) > 0 || len(next.Logs) > 0) {
		return errors.New("this connector can only output metrics")
	}

	if len(next.Metrics) > 0 {
		fanout := fanoutconsumer.Metrics(next.Metrics)
		metricsInterceptor := interceptconsumer.Metrics(fanout,
			func(ctx context.Context, md pmetric.Metrics) error {
				livedebuggingpublisher.PublishMetricsIfActive(p.debugDataPublisher, p.opts.ID, md, otelcol.GetComponentMetadata(next.Metrics))
				return fanout.ConsumeMetrics(ctx, md)
			},
		)

		// Create traces to metrics connector if supported
		if (connectorType & ConnectorTracesToMetrics) != 0 {
			tracesConnector, err = p.factory.CreateTracesToMetrics(p.ctx, settings, connectorConfig, metricsInterceptor)
			if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
				return err
			} else if tracesConnector != nil {
				components = append(components, tracesConnector)
			}
		}

		// Create metrics to metrics connector if supported
		if (connectorType & ConnectorMetricsToMetrics) != 0 {
			metricsConnector, err = p.factory.CreateMetricsToMetrics(p.ctx, settings, connectorConfig, metricsInterceptor)
			if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
				return err
			} else if metricsConnector != nil {
				components = append(components, metricsConnector)
			}
		}

		// Create logs to metrics connector if supported
		if (connectorType & ConnectorLogsToMetrics) != 0 {
			logsConnector, err = p.factory.CreateLogsToMetrics(p.ctx, settings, connectorConfig, metricsInterceptor)
			if err != nil && !errors.Is(err, pipeline.ErrSignalNotSupported) {
				return err
			} else if logsConnector != nil {
				components = append(components, logsConnector)
			}
		}
	}

	if len(components) == 0 {
		return errors.New("no connectors were created")
	}

	updateConsumersFunc := func() {
		p.consumer.SetConsumers(tracesConnector, metricsConnector, logsConnector)
	}

	// Schedule the components to run once our component is running.
	p.sched.Schedule(p.ctx, updateConsumersFunc, host, components...)
	return nil
}

// CurrentHealth implements component.HealthComponent.
func (p *Connector) CurrentHealth() component.Health {
	return p.sched.CurrentHealth()
}

func (p *Connector) LiveDebugging() {}
