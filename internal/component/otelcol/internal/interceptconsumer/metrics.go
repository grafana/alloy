package interceptconsumer

import (
	"context"

	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type MetricsInterceptorFunc func(context.Context, pmetric.Metrics) error

type MetricsInterceptor struct {
	onConsumeMetrics MetricsInterceptorFunc
	nextMetrics      otelconsumer.Metrics
	mutatesData      bool
}

// Use LogsMutating if the interceptor func is modifying the data
func Metrics(nextMetrics otelconsumer.Metrics, f MetricsInterceptorFunc) otelconsumer.Metrics {
	return &MetricsInterceptor{
		nextMetrics:      nextMetrics,
		mutatesData:      false,
		onConsumeMetrics: f,
	}
}

func MetricsMutating(nextMetrics otelconsumer.Metrics, f MetricsInterceptorFunc) otelconsumer.Metrics {
	return &MetricsInterceptor{
		nextMetrics:      nextMetrics,
		mutatesData:      true,
		onConsumeMetrics: f,
	}
}

func (i *MetricsInterceptor) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: i.mutatesData}
}

func (i *MetricsInterceptor) ConsumeMetrics(ctx context.Context, ld pmetric.Metrics) error {
	if i.onConsumeMetrics != nil {
		return i.onConsumeMetrics(ctx, ld)
	}

	return i.nextMetrics.ConsumeMetrics(ctx, ld)
}
