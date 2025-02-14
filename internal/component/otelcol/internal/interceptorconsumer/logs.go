package interceptorconsumer

import (
	"context"

	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
)

type LogsInterceptorFunc func(context.Context, plog.Logs) error

type LogsInterceptor struct {
	onConsumeLogs LogsInterceptorFunc
	nextLogs      otelconsumer.Logs
	mutatesData   bool // must be set to true if the provided opts modifies the data
}

func Logs(nextLogs otelconsumer.Logs, mutatesData bool, f LogsInterceptorFunc) otelconsumer.Logs {
	return &LogsInterceptor{
		nextLogs:      nextLogs,
		mutatesData:   mutatesData,
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
