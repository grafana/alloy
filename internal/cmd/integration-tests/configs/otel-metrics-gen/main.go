package main

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

const otelExporterOtlpEndpoint = "OTEL_EXPORTER_ENDPOINT"

func main() {
	ctx := context.Background()
	otlpExporterEndpoint, ok := os.LookupEnv(otelExporterOtlpEndpoint)
	if !ok {
		otlpExporterEndpoint = "localhost:4318"
	}
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithInsecure(),
		otlpmetrichttp.WithEndpoint(otlpExporterEndpoint),
	)
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	resource, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("otel-metrics-gen"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	exponentialHistogramView := metric.NewView(
		metric.Instrument{
			Name: "example_exponential_*",
		},
		metric.Stream{
			Aggregation: metric.AggregationBase2ExponentialHistogram{
				MaxSize:  160,
				MaxScale: 20,
			},
		},
	)

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(1*time.Second))),
		sdkmetric.WithResource(resource),
		metric.WithView(exponentialHistogramView),
	)
	otel.SetMeterProvider(provider)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := provider.Shutdown(ctx); err != nil {
			log.Fatalf("Server shutdown error: %v", err)
		}
	}()

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
		counter.Add(ctx, 10)
		floatCounter.Add(ctx, 2.5)
		upDownCounter.Add(ctx, -5)
		floatUpDownCounter.Add(ctx, 3.5)
		histogram.Record(ctx, 2)
		floatHistogram.Record(ctx, 6.5)
		exponentialHistogram.Record(ctx, 5)
		exponentialFloatHistogram.Record(ctx, 1.5)

		time.Sleep(200 * time.Millisecond)
	}
}
