package app_agent_receiver //nolint:golint

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/gorilla/mux"
	"github.com/grafana/agent/internal/static/integrations/v2"
	"github.com/grafana/agent/internal/static/integrations/v2/metricsutils"
	"github.com/grafana/agent/internal/static/traces/pushreceiver"
	"github.com/grafana/dskit/instrument"
	"github.com/grafana/dskit/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

type appAgentReceiverIntegration struct {
	integrations.MetricsIntegration
	appAgentReceiverHandler AppAgentReceiverHandler
	logger                  log.Logger
	conf                    *Config
	reg                     prometheus.Registerer

	requestDurationCollector     *prometheus.HistogramVec
	receivedMessageSizeCollector *prometheus.HistogramVec
	sentMessageSizeCollector     *prometheus.HistogramVec
	inflightRequestsCollector    *prometheus.GaugeVec
}

// Static typecheck tests
var (
	_ integrations.Integration        = (*appAgentReceiverIntegration)(nil)
	_ integrations.HTTPIntegration    = (*appAgentReceiverIntegration)(nil)
	_ integrations.MetricsIntegration = (*appAgentReceiverIntegration)(nil)
)

// NewIntegration converts this config into an instance of an integration
func (c *Config) NewIntegration(l log.Logger, globals integrations.Globals) (integrations.Integration, error) {
	reg := prometheus.NewRegistry()
	sourcemapLogger := log.With(l, "subcomponent", "sourcemaps")
	sourcemapStore := NewSourceMapStore(sourcemapLogger, c.SourceMaps, reg, nil, nil)

	receiverMetricsExporter := NewReceiverMetricsExporter(reg)

	var exp = []AppAgentReceiverExporter{
		receiverMetricsExporter,
	}

	if len(c.LogsInstance) > 0 {
		getLogsInstance := func() (logsInstance, error) {
			instance := globals.Logs.Instance(c.LogsInstance)
			if instance == nil {
				return nil, fmt.Errorf("logs instance \"%s\" not found", c.LogsInstance)
			}
			return instance, nil
		}

		if _, err := getLogsInstance(); err != nil {
			return nil, err
		}

		lokiExporter := NewLogsExporter(
			l,
			LogsExporterConfig{
				GetLogsInstance:  getLogsInstance,
				Labels:           c.LogsLabels,
				SendEntryTimeout: c.LogsSendTimeout,
			},
			sourcemapStore,
		)
		exp = append(exp, lokiExporter)
	}

	if len(c.TracesInstance) > 0 {
		getTracesConsumer := func() (consumer.Traces, error) {
			tracesInstance := globals.Tracing.Instance(c.TracesInstance)
			if tracesInstance == nil {
				return nil, fmt.Errorf("traces instance \"%s\" not found", c.TracesInstance)
			}
			factory := tracesInstance.GetFactory(component.KindReceiver, pushreceiver.TypeStr)
			if factory == nil {
				return nil, fmt.Errorf("push receiver factory not found for traces instance \"%s\"", c.TracesInstance)
			}
			consumer := factory.(*pushreceiver.Factory).Consumer
			if consumer == nil {
				return nil, fmt.Errorf("consumer not set for push receiver factory on traces instance \"%s\"", c.TracesInstance)
			}
			return consumer, nil
		}
		if _, err := getTracesConsumer(); err != nil {
			return nil, err
		}
		tracesExporter := NewTracesExporter(getTracesConsumer)
		exp = append(exp, tracesExporter)
	}

	handler := NewAppAgentReceiverHandler(c, exp, reg)

	metricsIntegration, err := metricsutils.NewMetricsHandlerIntegration(l, c, c.Common, globals, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	if err != nil {
		return nil, err
	}

	requestDurationCollector := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "app_agent_receiver_request_duration_seconds",
		Help:    "Time (in seconds) spent serving HTTP requests.",
		Buckets: instrument.DefBuckets,
	}, []string{"method", "route", "status_code", "ws"})
	reg.MustRegister(requestDurationCollector)

	receivedMessageSizeCollector := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "app_agent_receiver_request_message_bytes",
		Help:    "Size (in bytes) of messages received in the request.",
		Buckets: middleware.BodySizeBuckets,
	}, []string{"method", "route"})
	reg.MustRegister(receivedMessageSizeCollector)

	sentMessageSizeCollector := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "app_agent_receiver_response_message_bytes",
		Help:    "Size (in bytes) of messages sent in response.",
		Buckets: middleware.BodySizeBuckets,
	}, []string{"method", "route"})
	reg.MustRegister(sentMessageSizeCollector)

	inflightRequestsCollector := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_agent_receiver_inflight_requests",
		Help: "Current number of inflight requests.",
	}, []string{"method", "route"})
	reg.MustRegister(inflightRequestsCollector)

	return &appAgentReceiverIntegration{
		MetricsIntegration:      metricsIntegration,
		appAgentReceiverHandler: handler,
		logger:                  l,
		conf:                    c,
		reg:                     reg,

		requestDurationCollector:     requestDurationCollector,
		receivedMessageSizeCollector: receivedMessageSizeCollector,
		sentMessageSizeCollector:     sentMessageSizeCollector,
		inflightRequestsCollector:    inflightRequestsCollector,
	}, nil
}

// RunIntegration implements Integration
func (i *appAgentReceiverIntegration) RunIntegration(ctx context.Context) error {
	r := mux.NewRouter()
	r.Handle("/collect", i.appAgentReceiverHandler.HTTPHandler(i.logger)).Methods("POST", "OPTIONS")

	mw := middleware.Instrument{
		RouteMatcher:     r,
		Duration:         i.requestDurationCollector,
		RequestBodySize:  i.receivedMessageSizeCollector,
		ResponseBodySize: i.sentMessageSizeCollector,
		InflightRequests: i.inflightRequestsCollector,
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", i.conf.Server.Host, i.conf.Server.Port),
		Handler: mw.Wrap(r),
	}
	errChan := make(chan error, 1)

	go func() {
		level.Info(i.logger).Log("msg", "starting app agent receiver", "host", i.conf.Server.Host, "port", i.conf.Server.Port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		if err := srv.Shutdown(ctx); err != nil {
			return err
		}
	case err := <-errChan:
		close(errChan)
		return err
	}

	return nil
}

func init() {
	integrations.Register(&Config{}, integrations.TypeMultiplex)
}
