package traces

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	otelexporter "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/grafana/agent/internal/build"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/static/traces/automaticloggingprocessor"
	"github.com/grafana/agent/internal/static/traces/contextkeys"
	"github.com/grafana/agent/internal/static/traces/servicegraphprocessor"
	"github.com/grafana/agent/internal/static/traces/traceutils"
	"github.com/grafana/agent/internal/util"
	prom_client "github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace/noop"
)

// Instance wraps the OpenTelemetry collector to enable tracing pipelines
type Instance struct {
	mut    sync.Mutex
	cfg    InstanceConfig
	logger *zap.Logger

	factories otelcol.Factories
	service   *service.Service
}

// NewInstance creates and starts an instance of tracing pipelines.
func NewInstance(logsSubsystem *logs.Logs, reg prom_client.Registerer, cfg InstanceConfig, logger *zap.Logger, promInstanceManager instance.Manager) (*Instance, error) {
	instance := &Instance{}
	instance.logger = logger

	if err := instance.ApplyConfig(logsSubsystem, promInstanceManager, reg, cfg); err != nil {
		return nil, err
	}
	return instance, nil
}

// ApplyConfig updates the configuration of the Instance.
func (i *Instance) ApplyConfig(logsSubsystem *logs.Logs, promInstanceManager instance.Manager, reg prom_client.Registerer, cfg InstanceConfig) error {
	i.mut.Lock()
	defer i.mut.Unlock()

	if util.CompareYAML(cfg, i.cfg) {
		// No config change
		return nil
	}
	i.cfg = cfg

	// Shut down any existing pipeline
	i.stop()

	err := i.buildAndStartPipeline(context.Background(), cfg, logsSubsystem, promInstanceManager, reg)
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}

	return nil
}

// Stop stops the OpenTelemetry collector subsystem
func (i *Instance) Stop() {
	i.mut.Lock()
	defer i.mut.Unlock()

	i.stop()
}

func (i *Instance) stop() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if i.service != nil {
		err := i.service.Shutdown(shutdownCtx)
		if err != nil {
			i.logger.Error("failed to stop Otel service", zap.Error(err))
		}
	}
}

func (i *Instance) buildAndStartPipeline(ctx context.Context, cfg InstanceConfig, logs *logs.Logs, instManager instance.Manager, reg prom_client.Registerer) error {
	// create component factories
	otelConfig, err := cfg.OtelConfig()
	if err != nil {
		return fmt.Errorf("failed to load otelConfig from agent traces config: %w", err)
	}
	for _, rw := range cfg.RemoteWrite {
		if rw.InsecureSkipVerify {
			i.logger.Warn("Configuring TLS with insecure_skip_verify. Use tls_config.insecure_skip_verify instead")
		}
		if rw.TLSConfig != nil && rw.TLSConfig.ServerName != "" {
			i.logger.Warn("Configuring unsupported tls_config.server_name")
		}
	}

	if cfg.SpanMetrics != nil && len(cfg.SpanMetrics.MetricsInstance) != 0 {
		ctx = context.WithValue(ctx, contextkeys.Metrics, instManager)
	}

	if cfg.LoadBalancing == nil && (cfg.TailSampling != nil || cfg.ServiceGraphs != nil) {
		i.logger.Warn("Configuring tail_sampling and/or service_graphs without load_balancing." +
			"Load balancing via trace ID is required for those features to work properly in multi agent deployments")
	}

	if cfg.LoadBalancing == nil && cfg.SpanMetrics != nil {
		i.logger.Warn("Configuring spanmetrics without load_balancing." +
			"Load balancing via service name is required for spanmetrics to work properly in multi agent deployments")
	}

	if cfg.AutomaticLogging != nil && cfg.AutomaticLogging.Backend != automaticloggingprocessor.BackendStdout {
		ctx = context.WithValue(ctx, contextkeys.Logs, logs)
	}

	factories, err := tracingFactories()
	if err != nil {
		return fmt.Errorf("failed to load tracing factories: %w", err)
	}
	i.factories = factories

	appinfo := component.BuildInfo{
		Command:     "agent",
		Description: "agent",
		Version:     build.Version,
	}

	err = util.SetupStaticModeOtelFeatureGates()
	if err != nil {
		return err
	}

	promExporter, err := traceutils.PrometheusExporter(reg)
	if err != nil {
		return fmt.Errorf("error creating otel prometheus exporter: %w", err)
	}

	i.service, err = service.New(ctx, service.Settings{
		BuildInfo:                appinfo,
		Receivers:                receiver.NewBuilder(otelConfig.Receivers, i.factories.Receivers),
		Processors:               processor.NewBuilder(otelConfig.Processors, i.factories.Processors),
		Exporters:                otelexporter.NewBuilder(otelConfig.Exporters, i.factories.Exporters),
		Connectors:               connector.NewBuilder(otelConfig.Connectors, i.factories.Connectors),
		Extensions:               extension.NewBuilder(otelConfig.Extensions, i.factories.Extensions),
		OtelMetricViews:          servicegraphprocessor.OtelMetricViews(),
		OtelMetricReader:         promExporter,
		DisableProcessMetrics:    true,
		UseExternalMetricsServer: true,
		TracerProvider:           noop.NewTracerProvider(),
		//TODO: Plug in an AsyncErrorChannel to shut down the Agent in case of a fatal event
		LoggingOptions: []zap.Option{
			zap.WrapCore(func(zapcore.Core) zapcore.Core {
				return i.logger.Core()
			}),
		},
	}, otelConfig.Service)
	if err != nil {
		return fmt.Errorf("failed to create Otel service: %w", err)
	}

	err = i.service.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start Otel service: %w", err)
	}

	return err
}

// ReportFatalError implements component.Host
func (i *Instance) ReportFatalError(err error) {
	i.logger.Error("fatal error reported", zap.Error(err))
}

// GetFactory implements component.Host
func (i *Instance) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	switch kind {
	case component.KindReceiver:
		return i.factories.Receivers[componentType]
	default:
		return nil
	}
}
