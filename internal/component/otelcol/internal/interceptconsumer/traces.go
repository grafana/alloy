package interceptconsumer

import (
	"context"

	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type TracesInterceptorFunc func(context.Context, ptrace.Traces) error

type TracesInterceptor struct {
	onConsumeTraces TracesInterceptorFunc
	nextTraces      otelconsumer.Traces
	mutatesData     bool
}

// Use LogsMutating if the interceptor func is modifying the data
func Traces(nextTraces otelconsumer.Traces, f TracesInterceptorFunc) otelconsumer.Traces {
	return &TracesInterceptor{
		nextTraces:      nextTraces,
		mutatesData:     false,
		onConsumeTraces: f,
	}
}

func TracesMutating(nextTraces otelconsumer.Traces, f TracesInterceptorFunc) otelconsumer.Traces {
	return &TracesInterceptor{
		nextTraces:      nextTraces,
		mutatesData:     true,
		onConsumeTraces: f,
	}
}

func (i *TracesInterceptor) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: i.mutatesData}
}

func (i *TracesInterceptor) ConsumeTraces(ctx context.Context, ld ptrace.Traces) error {
	if i.onConsumeTraces != nil {
		return i.onConsumeTraces(ctx, ld)
	}

	return i.nextTraces.ConsumeTraces(ctx, ld)
}
