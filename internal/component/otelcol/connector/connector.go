// Package connector exposes utilities to create an Alloy component from
// OpenTelemetry Collector connectors.
package connector

import (
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconnector "go.opentelemetry.io/collector/connector"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
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
	"github.com/grafana/alloy/internal/slogadapter"
)

// Each constant must occupy a distinct bit: connectors combine them with the
// bitwise OR operator to declare the signal pairs they support.
const (
	ConnectorTracesToTraces = 1 << iota
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
			Logger:         slogadapter.NewZap(p.opts.Logger),
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

	// Create instances of the connector from our factory, one per supported
	// (input signal, output signal) pair.
	var components []otelcomponent.Component

	var tracesConnectors []otelconsumer.Traces
	var metricsConnectors []otelconsumer.Metrics
	var logsConnectors []otelconsumer.Logs

	connectorType := p.args.ConnectorType()

	// Validate that the connector supports the requested output types
	if !outputsToMetrics(connectorType) && len(next.Metrics) > 0 {
		return errors.New("this connector cannot output metrics")
	}

	if !outputsToLogs(connectorType) && len(next.Logs) > 0 {
		return errors.New("this connector cannot output logs")
	}

	if !outputsToTraces(connectorType) && len(next.Traces) > 0 {
		return errors.New("this connector cannot output traces")
	}

	// addConnector records a connector instance created by the factory. A
	// factory which lacks support for a signal pair returns
	// pipeline.ErrSignalNotSupported, which is tolerated.
	addConnector := func(c otelcomponent.Component, err error) (bool, error) {
		if err != nil {
			if errors.Is(err, pipeline.ErrSignalNotSupported) {
				return false, nil
			}
			return false, err
		}
		if c == nil {
			return false, nil
		}
		components = append(components, c)
		return true, nil
	}

	if len(next.Metrics) > 0 {
		fanout := fanoutconsumer.Metrics(next.Metrics)
		metricsInterceptor := interceptconsumer.Metrics(fanout,
			func(ctx context.Context, md pmetric.Metrics) error {
				livedebuggingpublisher.PublishMetricsIfActive(p.debugDataPublisher, p.opts.ID, md, otelcol.GetComponentMetadata(next.Metrics))
				return fanout.ConsumeMetrics(ctx, md)
			},
		)

		if (connectorType & ConnectorTracesToMetrics) != 0 {
			c, err := p.factory.CreateTracesToMetrics(p.ctx, settings, connectorConfig, metricsInterceptor)
			if ok, err := addConnector(c, err); err != nil {
				return err
			} else if ok {
				tracesConnectors = append(tracesConnectors, c)
			}
		}

		if (connectorType & ConnectorMetricsToMetrics) != 0 {
			c, err := p.factory.CreateMetricsToMetrics(p.ctx, settings, connectorConfig, metricsInterceptor)
			if ok, err := addConnector(c, err); err != nil {
				return err
			} else if ok {
				metricsConnectors = append(metricsConnectors, c)
			}
		}

		if (connectorType & ConnectorLogsToMetrics) != 0 {
			c, err := p.factory.CreateLogsToMetrics(p.ctx, settings, connectorConfig, metricsInterceptor)
			if ok, err := addConnector(c, err); err != nil {
				return err
			} else if ok {
				logsConnectors = append(logsConnectors, c)
			}
		}
	}

	if len(next.Logs) > 0 {
		fanout := fanoutconsumer.Logs(next.Logs)
		logsInterceptor := interceptconsumer.Logs(fanout,
			func(ctx context.Context, ld plog.Logs) error {
				livedebuggingpublisher.PublishLogsIfActive(p.debugDataPublisher, p.opts.ID, ld, otelcol.GetComponentMetadata(next.Logs))
				return fanout.ConsumeLogs(ctx, ld)
			},
		)

		if (connectorType & ConnectorTracesToLogs) != 0 {
			c, err := p.factory.CreateTracesToLogs(p.ctx, settings, connectorConfig, logsInterceptor)
			if ok, err := addConnector(c, err); err != nil {
				return err
			} else if ok {
				tracesConnectors = append(tracesConnectors, c)
			}
		}

		if (connectorType & ConnectorMetricsToLogs) != 0 {
			c, err := p.factory.CreateMetricsToLogs(p.ctx, settings, connectorConfig, logsInterceptor)
			if ok, err := addConnector(c, err); err != nil {
				return err
			} else if ok {
				metricsConnectors = append(metricsConnectors, c)
			}
		}

		if (connectorType & ConnectorLogsToLogs) != 0 {
			c, err := p.factory.CreateLogsToLogs(p.ctx, settings, connectorConfig, logsInterceptor)
			if ok, err := addConnector(c, err); err != nil {
				return err
			} else if ok {
				logsConnectors = append(logsConnectors, c)
			}
		}
	}

	if len(components) == 0 {
		return errors.New("no connectors were created")
	}

	// A nil consumer makes the lazy consumer reject the signal with
	// pipeline.ErrSignalNotSupported.
	var tracesConnector otelconsumer.Traces
	if len(tracesConnectors) > 0 {
		tracesConnector = fanoutconsumer.TracesFromConsumers(tracesConnectors)
	}
	var metricsConnector otelconsumer.Metrics
	if len(metricsConnectors) > 0 {
		metricsConnector = fanoutconsumer.MetricsFromConsumers(metricsConnectors)
	}
	var logsConnector otelconsumer.Logs
	if len(logsConnectors) > 0 {
		logsConnector = fanoutconsumer.LogsFromConsumers(logsConnectors)
	}

	updateConsumersFunc := func() {
		p.consumer.SetConsumers(tracesConnector, metricsConnector, logsConnector, nil)
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

// outputsToMetrics checks if the connector can output metrics by testing if any of the
// *ToMetrics flags are set in the connectorType.
func outputsToMetrics(connectorType int) bool {
	return (connectorType&ConnectorTracesToMetrics) != 0 ||
		(connectorType&ConnectorMetricsToMetrics) != 0 ||
		(connectorType&ConnectorLogsToMetrics) != 0
}

// outputsToLogs checks if the connector can output logs by testing if any of the
// *ToLogs flags are set in the connectorType.
func outputsToLogs(connectorType int) bool {
	return (connectorType&ConnectorTracesToLogs) != 0 ||
		(connectorType&ConnectorMetricsToLogs) != 0 ||
		(connectorType&ConnectorLogsToLogs) != 0
}

// outputsToTraces checks if the connector can output traces by testing if any of the
// *ToTraces flags are set in the connectorType.
func outputsToTraces(connectorType int) bool {
	return (connectorType&ConnectorTracesToTraces) != 0 ||
		(connectorType&ConnectorMetricsToTraces) != 0 ||
		(connectorType&ConnectorLogsToTraces) != 0
}
