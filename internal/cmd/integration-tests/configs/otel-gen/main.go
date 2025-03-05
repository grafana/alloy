package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	tr "go.opentelemetry.io/otel/trace"
)

const otelExporterOtlpEndpoint = "OTEL_EXPORTER_ENDPOINT"

func main() {
	ctx := context.Background()
	otlpExporterEndpoint, ok := os.LookupEnv(otelExporterOtlpEndpoint)
	if !ok {
		otlpExporterEndpoint = "localhost:4318"
	}

	// Setting up the trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithEndpoint(otlpExporterEndpoint),
	)
	if err != nil {
		log.Fatalf("failed to create trace exporter: %v", err)
	}

	// Setting up the metric exporter
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithEndpoint(otlpExporterEndpoint),
	)
	if err != nil {
		log.Fatalf("failed to create metric exporter: %v", err)
	}

	// Create a resource for the client service
	clientResource, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("client-service"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create client resource: %v", err)
	}

	// Create a resource for the server service
	serverResource, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("server-service"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create server resource: %v", err)
	}

	// Create client tracer provider with explicit batch settings
	clientTP := trace.NewTracerProvider(
		trace.WithBatcher(
			traceExporter,
			// Set a shorter batching interval (default is 5s)
			trace.WithBatchTimeout(100*time.Millisecond),
		),
		trace.WithResource(clientResource),
	)

	// Create server tracer provider with explicit batch settings
	serverTP := trace.NewTracerProvider(
		trace.WithBatcher(
			traceExporter,
			// Set a shorter batching interval (default is 5s)
			trace.WithBatchTimeout(100*time.Millisecond),
		),
		trace.WithResource(serverResource),
	)

	// Set the global tracer provider to the client one (we'll use the server one explicitly)
	otel.SetTracerProvider(clientTP)

	defer func() {
		if err := clientTP.Shutdown(ctx); err != nil {
			log.Printf("failed to shut down client tracer provider: %v", err)
		}
		if err := serverTP.Shutdown(ctx); err != nil {
			log.Printf("failed to shut down server tracer provider: %v", err)
		}
	}()

	exponentialHistogramView := sdkmetric.NewView(
		sdkmetric.Instrument{
			Name: "example_exponential_*",
		},
		sdkmetric.Stream{
			Aggregation: sdkmetric.AggregationBase2ExponentialHistogram{
				MaxSize:  160,
				MaxScale: 20,
			},
		},
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(
			metricExporter,
			sdkmetric.WithInterval(100*time.Millisecond))),
		sdkmetric.WithResource(serverResource),
		sdkmetric.WithView(exponentialHistogramView),
	)
	otel.SetMeterProvider(provider)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := provider.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}()

	// Get tracers for both client and server
	clientTracer := clientTP.Tracer("client-tracer")
	serverTracer := serverTP.Tracer("server-tracer")

	meter := otel.Meter("example-meter")
	counter, _ := meter.Int64Counter("example_counter")
	floatCounter, _ := meter.Float64Counter("example_float_counter")
	upDownCounter, _ := meter.Int64UpDownCounter("example_updowncounter")
	floatUpDownCounter, _ := meter.Float64UpDownCounter("example_float_updowncounter")
	histogram, _ := meter.Int64Histogram("example_histogram")
	floatHistogram, _ := meter.Float64Histogram("example_float_histogram")
	exponentialHistogram, _ := meter.Int64Histogram("example_exponential_histogram")
	exponentialFloatHistogram, _ := meter.Float64Histogram("example_exponential_float_histogram")

	for {
		// Start a client span
		ctx, clientSpan := clientTracer.Start(
			ctx,
			"client-request",
			tr.WithSpanKind(tr.SpanKindClient),
			tr.WithAttributes(
				semconv.NetPeerNameKey.String("server-service"),
				semconv.NetPeerPortKey.Int(8080),
			),
		)

		// Start a server span as a child of the client span
		ctx, serverSpan := serverTracer.Start(
			ctx,
			"server-handler",
			tr.WithSpanKind(tr.SpanKindServer),
			tr.WithAttributes(
				semconv.NetHostNameKey.String("server-service"),
				semconv.NetHostPortKey.Int(8080),
			),
		)

		counter.Add(ctx, 10)
		floatCounter.Add(ctx, 2.5)
		upDownCounter.Add(ctx, -5)
		floatUpDownCounter.Add(ctx, 3.5)
		histogram.Record(ctx, 2)
		floatHistogram.Record(ctx, 6.5)
		exponentialHistogram.Record(ctx, 5)
		exponentialFloatHistogram.Record(ctx, 1.5)

		serverSpan.End()
		clientSpan.End()

		time.Sleep(50 * time.Millisecond)
	}
}
