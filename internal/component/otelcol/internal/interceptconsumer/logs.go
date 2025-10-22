package interceptconsumer

import (
	"context"

	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

type LogsInterceptorFunc func(context.Context, plog.Logs) error

type LogsInterceptor struct {
	onConsumeLogs LogsInterceptorFunc
	nextLogs      otelconsumer.Logs
	mutatesData   bool
}

// Use LogsMutating if the interceptor func is modifying the data
func Logs(nextLogs otelconsumer.Logs, f LogsInterceptorFunc) otelconsumer.Logs {
	return &LogsInterceptor{
		nextLogs:      nextLogs,
		mutatesData:   false,
		onConsumeLogs: f,
	}
}

func LogsMutating(nextLogs otelconsumer.Logs, f LogsInterceptorFunc) otelconsumer.Logs {
	return &LogsInterceptor{
		nextLogs:      nextLogs,
		mutatesData:   true,
		onConsumeLogs: f,
	}
}

func (i *LogsInterceptor) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: i.mutatesData}
}

func (i *LogsInterceptor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if i.onConsumeLogs != nil {
		return i.onConsumeLogs(ctx, ld)
	}

	return i.nextLogs.ConsumeLogs(ctx, ld)
}
