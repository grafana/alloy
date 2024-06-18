// Package tracing implements the tracing subsystem of Grafana Alloy. The
// tracing subsystem exposes a [trace.TraceProvider] which accepts traces and
// forwards them to a running component for further processing.
package tracing

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/build"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/runtime/tracing/internal/jaegerremote"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "alloy"

// Defaults for all Options structs.
var (
	DefaultOptions = Options{
		SamplingFraction: 0.1,                  // Keep 10% of spans
		WriteTo:          []otelcol.Consumer{}, // Don't send spans anywhere.
	}

	DefaultJaegerRemoteSamplerOptions = JaegerRemoteSamplerOptions{
		URL:             "http://127.0.0.1:5778/sampling",
		MaxOperations:   256,
		RefreshInterval: time.Minute,
	}
)

// Options control the tracing subsystem.
type Options struct {
	// SamplingFraction determines which rate of traces to sample. A value of 1
	// means to keep 100% of traces. A value of 0 means to keep 0% of traces.
	SamplingFraction float64 `alloy:"sampling_fraction,attr,optional"`

	// Sampler holds optional samplers to configure on top of the sampling
	// fraction.
	Sampler SamplerOptions `alloy:"sampler,block,optional"`

	// WriteTo holds a set of OpenTelemetry Collector consumers where internal
	// traces should be sent.
	WriteTo []otelcol.Consumer `alloy:"write_to,attr,optional"`
}

type SamplerOptions struct {
	JaegerRemote *JaegerRemoteSamplerOptions `alloy:"jaeger_remote,block,optional"`

	// TODO(rfratto): if support for another sampler is added, SamplerOptions
	// must enforce that only one inner block is provided.
}

type JaegerRemoteSamplerOptions struct {
	URL             string        `alloy:"url,attr,optional"`
	MaxOperations   int           `alloy:"max_operations,attr,optional"`
	RefreshInterval time.Duration `alloy:"refresh_interval,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (opts *Options) SetToDefault() {
	*opts = DefaultOptions
}

// SetToDefault implements syntax.Defaulter.
func (opts *JaegerRemoteSamplerOptions) SetToDefault() {
	*opts = DefaultJaegerRemoteSamplerOptions
}

// Tracer is the tracing subsystem of Grafana Alloy. It implements
// [trace.TracerProvider] and can be used to forward internally generated
// traces to a OpenTelemetry Collector-compatible Alloy component.
type Tracer struct {
	trace.TracerProvider
	sampler *lazySampler
	client  *client
	exp     *otlptrace.Exporter
	tp      *tracesdk.TracerProvider

	samplerMut          sync.Mutex
	jaegerRemoteSampler *jaegerremote.Sampler // In-use jaeger remote sampler (may be nil).
}

var _ trace.TracerProvider = (*Tracer)(nil)

// New creates a new tracing subsystem. Call Run to start the tracing
// subsystem.
func New(cfg Options) (*Tracer, error) {
	res, err := resource.New(
		context.Background(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(build.Version),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	// Create a lazy sampler and pre-seed it with the sampling fraction.
	var sampler lazySampler
	sampler.SetSampler(tracesdk.TraceIDRatioBased(cfg.SamplingFraction))

	shimClient := &client{}
	exp := otlptrace.NewUnstarted(shimClient)

	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exp),
		tracesdk.WithSampler(tracesdk.ParentBased(&sampler)),
		tracesdk.WithResource(res),
	)

	t := &Tracer{
		sampler: &sampler,
		client:  shimClient,
		exp:     exp,
		tp:      tp,
	}

	if err := t.Update(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

// Update provides a new config to the tracing subsystem.
func (t *Tracer) Update(opts Options) error {
	t.samplerMut.Lock()
	defer t.samplerMut.Unlock()

	t.client.UpdateWriteTo(opts.WriteTo)

	// Stop the previous instance of the Jaeger remote sampler if it exists. The
	// sampler can still make sampling decisions after being closed; it just
	// won't poll anymore.
	if t.jaegerRemoteSampler != nil {
		t.jaegerRemoteSampler.Close()
		t.jaegerRemoteSampler = nil
	}

	// Remote samplers accept a "seed" sampler to use before the remote is
	// available. Get the current sampler from the previous iteration.
	lastSampler := t.sampler.Sampler()

	switch {
	case opts.Sampler.JaegerRemote != nil:
		t.jaegerRemoteSampler = jaegerremote.New(
			serviceName,
			jaegerremote.WithSamplingServerURL(opts.Sampler.JaegerRemote.URL),
			jaegerremote.WithSamplingRefreshInterval(opts.Sampler.JaegerRemote.RefreshInterval),
			jaegerremote.WithMaxOperations(opts.Sampler.JaegerRemote.MaxOperations),
			jaegerremote.WithInitialSampler(lastSampler),
		)

		t.sampler.SetSampler(t.jaegerRemoteSampler)

	default:
		t.sampler.SetSampler(tracesdk.TraceIDRatioBased(opts.SamplingFraction))
	}

	return nil
}

// Run starts the tracing subsystem and runs it until the provided context is
// canceled. If the tracing subsystem could not be started, an error is
// returned.
//
// Run returns no error upon normal shutdown.
func (t *Tracer) Run(ctx context.Context) error {
	if err := t.exp.Start(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.tp.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

func (t *Tracer) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	return t.tp.Tracer(name, options...)
}
