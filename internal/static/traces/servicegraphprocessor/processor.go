package servicegraphprocessor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	util "github.com/grafana/alloy/internal/util/log"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	otelprocessor "go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.6.1"
	"google.golang.org/grpc/codes"
)

const (
	serviceGraphRequestTotal_name           = "service_graph_request"
	serviceGraphRequestFailedTotal_name     = "service_graph_request_failed"
	serviceGraphRequestServerHistogram_name = "service_graph_request_server_seconds"
	serviceGraphRequestClientHistogram_name = "service_graph_request_client_seconds"
	serviceGraphUnpairedSpansTotal_name     = "service_graph_unpaired_spans"
	serviceGraphDroppedSpansTotal_name      = "service_graph_dropped_spans"
)

type tooManySpansError struct {
	droppedSpans int
}

func (t tooManySpansError) Error() string {
	return fmt.Sprintf("dropped %d spans", t.droppedSpans)
}

// edge is an edge between two nodes in the graph
type edge struct {
	key string

	serverService, clientService string
	serverLatency, clientLatency time.Duration

	// If either the client or the server spans have status code error,
	// the edge will be considered as failed.
	failed bool

	// expiration is the time at which the edge expires, expressed as Unix time
	expiration int64
}

func newEdge(key string, ttl time.Duration) *edge {
	return &edge{
		key: key,

		expiration: time.Now().Add(ttl).Unix(),
	}
}

// isCompleted returns true if the corresponding client and server
// pair spans have been processed for the given edge
func (e *edge) isCompleted() bool {
	return len(e.clientService) != 0 && len(e.serverService) != 0
}

func (e *edge) isExpired() bool {
	return time.Now().Unix() >= e.expiration
}

var _ otelprocessor.Traces = (*processor)(nil)

type processor struct {
	nextConsumer consumer.Traces

	store *store

	wait     time.Duration
	maxItems int

	// completed edges are pushed through this channel to be processed.
	collectCh chan string

	serviceGraphRequestTotal           metric.Float64Counter
	serviceGraphRequestFailedTotal     metric.Float64Counter
	serviceGraphRequestServerHistogram metric.Float64Histogram
	serviceGraphRequestClientHistogram metric.Float64Histogram
	serviceGraphUnpairedSpansTotal     metric.Float64Counter
	serviceGraphDroppedSpansTotal      metric.Float64Counter

	httpSuccessCodeMap map[int]struct{}
	grpcSuccessCodeMap map[int]struct{}

	logger  log.Logger
	closeCh chan struct{}
}

func newProcessor(nextConsumer consumer.Traces, cfg *Config, set otelprocessor.Settings) (*processor, error) {
	logger := log.With(util.Logger, "component", "service graphs")

	if cfg.Wait == 0 {
		cfg.Wait = DefaultWait
	}
	if cfg.MaxItems == 0 {
		cfg.MaxItems = DefaultMaxItems
	}
	if cfg.Workers == 0 {
		cfg.Workers = DefaultWorkers
	}

	var (
		httpSuccessCodeMap = make(map[int]struct{})
		grpcSuccessCodeMap = make(map[int]struct{})
	)
	if cfg.SuccessCodes != nil {
		for _, sc := range cfg.SuccessCodes.http {
			httpSuccessCodeMap[int(sc)] = struct{}{}
		}
		for _, sc := range cfg.SuccessCodes.grpc {
			grpcSuccessCodeMap[int(sc)] = struct{}{}
		}
	}

	p := &processor{
		nextConsumer: nextConsumer,
		logger:       logger,

		wait:               cfg.Wait,
		maxItems:           cfg.MaxItems,
		httpSuccessCodeMap: httpSuccessCodeMap,
		grpcSuccessCodeMap: grpcSuccessCodeMap,

		collectCh: make(chan string, cfg.Workers),

		closeCh: make(chan struct{}, 1),
	}

	for i := 0; i < cfg.Workers; i++ {
		go func() {
			for {
				select {
				case k := <-p.collectCh:
					p.store.evictEdgeWithLock(k)

				case <-p.closeCh:
					return
				}
			}
		}()
	}

	err := p.registerMetrics(set.MeterProvider, set.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to register service graph metrics: %w", err)
	}

	return p, nil
}

func (p *processor) Start(_ context.Context, _ component.Host) error {
	// initialize store
	p.store = newStore(p.wait, p.maxItems, p.collectEdge)

	return nil
}

func (p *processor) registerMetrics(mp metric.MeterProvider, meterId string) error {
	meter := mp.Meter(meterId)

	var err error
	p.serviceGraphRequestTotal, err = meter.Float64Counter(
		serviceGraphRequestTotal_name,
		metric.WithDescription("Total count of requests between two nodes"),
	)
	if err != nil {
		return err
	}

	p.serviceGraphRequestFailedTotal, err = meter.Float64Counter(
		serviceGraphRequestFailedTotal_name,
		metric.WithDescription("Total count of failed requests between two nodes"),
	)
	if err != nil {
		return err
	}

	p.serviceGraphRequestServerHistogram, err = meter.Float64Histogram(
		serviceGraphRequestServerHistogram_name,
		metric.WithDescription("Time for a request between two nodes as seen from the server"),
	)
	if err != nil {
		return err
	}

	p.serviceGraphRequestClientHistogram, err = meter.Float64Histogram(
		serviceGraphRequestClientHistogram_name,
		metric.WithDescription("Time for a request between two nodes as seen from the client"),
	)
	if err != nil {
		return err
	}

	p.serviceGraphUnpairedSpansTotal, err = meter.Float64Counter(
		serviceGraphUnpairedSpansTotal_name,
		metric.WithDescription("Total count of unpaired spans"),
	)
	if err != nil {
		return err
	}

	p.serviceGraphDroppedSpansTotal, err = meter.Float64Counter(
		serviceGraphDroppedSpansTotal_name,
		metric.WithDescription("Total count of dropped spans"),
	)
	if err != nil {
		return err
	}

	return nil
}

func OtelMetricViews() []sdkmetric.View {
	return []sdkmetric.View{
		sdkmetric.NewView(
			sdkmetric.Instrument{Name: serviceGraphRequestServerHistogram_name},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: []float64{0.01, 0.02, 0.04, 0.08, 0.16, 0.32, 0.64, 1.28, 2.56, 5.12, 10.24, 20.48},
			}},
		),
		sdkmetric.NewView(
			sdkmetric.Instrument{Name: serviceGraphRequestClientHistogram_name},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: []float64{0.01, 0.02, 0.04, 0.08, 0.16, 0.32, 0.64, 1.28, 2.56, 5.12, 10.24, 20.48},
			}},
		),
	}
}

func (p *processor) Shutdown(context.Context) error {
	close(p.closeCh)
	return nil
}

func (p *processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (p *processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Evict expired edges
	p.store.expire()

	if err := p.consume(td); err != nil {
		if errors.As(err, &tooManySpansError{}) {
			level.Warn(p.logger).Log("msg", "skipped processing of spans", "maxItems", p.maxItems, "err", err)
		} else {
			level.Error(p.logger).Log("msg", "failed consuming traces", "err", err)
		}
		return nil
	}

	return p.nextConsumer.ConsumeTraces(ctx, td)
}

// collectEdge records the metrics for the given edge.
// Returns true if the edge is completed or expired and should be deleted.
func (p *processor) collectEdge(e *edge) {
	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.Key("client").String(e.clientService),
		attribute.Key("server").String(e.serverService),
	}

	if e.isCompleted() {
		p.serviceGraphRequestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		if e.failed {
			p.serviceGraphRequestFailedTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		p.serviceGraphRequestServerHistogram.Record(ctx, e.serverLatency.Seconds(), metric.WithAttributes(attrs...))
		p.serviceGraphRequestClientHistogram.Record(ctx, e.clientLatency.Seconds(), metric.WithAttributes(attrs...))
	} else if e.isExpired() {
		p.serviceGraphUnpairedSpansTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

func (p *processor) consume(trace ptrace.Traces) error {
	var totalDroppedSpans int
	rSpansSlice := trace.ResourceSpans()
	for i := 0; i < rSpansSlice.Len(); i++ {
		rSpan := rSpansSlice.At(i)

		svc, ok := rSpan.Resource().Attributes().Get(string(semconv.ServiceNameKey))
		if !ok || svc.Str() == "" {
			continue
		}

		ctx := context.Background()
		attrsClient := []attribute.KeyValue{
			attribute.Key("client").String(svc.Str()),
			attribute.Key("server").String(""),
		}
		attrsServer := []attribute.KeyValue{
			attribute.Key("client").String(""),
			attribute.Key("server").String(svc.Str()),
		}

		ssSlice := rSpan.ScopeSpans()
		for j := 0; j < ssSlice.Len(); j++ {
			ils := ssSlice.At(j)

			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)

				switch span.Kind() {
				case ptrace.SpanKindClient:
					k := key(span.TraceID().String(), span.SpanID().String())

					edge, err := p.store.upsertEdge(k, func(e *edge) {
						e.clientService = svc.Str()
						e.clientLatency = spanDuration(span)
						e.failed = e.failed || p.spanFailed(span) // keep request as failed if any span is failed
					})

					if errors.Is(err, errTooManyItems) {
						totalDroppedSpans++
						p.serviceGraphDroppedSpansTotal.Add(ctx, 1, metric.WithAttributes(attrsClient...))
						continue
					}
					// upsertEdge will only return this errTooManyItems
					if err != nil {
						return err
					}

					if edge.isCompleted() {
						p.collectCh <- k
					}

				case ptrace.SpanKindServer:
					k := key(span.TraceID().String(), span.ParentSpanID().String())

					edge, err := p.store.upsertEdge(k, func(e *edge) {
						e.serverService = svc.Str()
						e.serverLatency = spanDuration(span)
						e.failed = e.failed || p.spanFailed(span) // keep request as failed if any span is failed
					})

					if errors.Is(err, errTooManyItems) {
						totalDroppedSpans++
						p.serviceGraphDroppedSpansTotal.Add(ctx, 1, metric.WithAttributes(attrsServer...))
						continue
					}
					// upsertEdge will only return this errTooManyItems
					if err != nil {
						return err
					}

					if edge.isCompleted() {
						p.collectCh <- k
					}

				default:
				}
			}
		}
	}

	if totalDroppedSpans > 0 {
		return &tooManySpansError{
			droppedSpans: totalDroppedSpans,
		}
	}
	return nil
}

func (p *processor) spanFailed(span ptrace.Span) bool {
	// Request considered failed if status is not 2XX or added as a successful status code
	if statusCode, ok := span.Attributes().Get(string(semconv.HTTPStatusCodeKey)); ok {
		sc := int(statusCode.Int())
		if _, ok := p.httpSuccessCodeMap[sc]; !ok && sc/100 != 2 {
			return true
		}
	}

	// Request considered failed if status is not OK or added as a successful status code
	if statusCode, ok := span.Attributes().Get(string(semconv.RPCGRPCStatusCodeKey)); ok {
		sc := int(statusCode.Int())
		if _, ok := p.grpcSuccessCodeMap[sc]; !ok && sc != int(codes.OK) {
			return true
		}
	}

	return span.Status().Code() == ptrace.StatusCodeError
}

func spanDuration(span ptrace.Span) time.Duration {
	return span.EndTimestamp().AsTime().Sub(span.StartTimestamp().AsTime())
}

func key(k1, k2 string) string {
	return fmt.Sprintf("%s-%s", k1, k2)
}
