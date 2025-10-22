package livedebuggingpublisher

import (
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/textmarshaler"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func PublishLogsIfActive(debugDataPublisher livedebugging.DebugDataPublisher, componentID string, ld plog.Logs, nextLogs []otelcol.ComponentMetadata) {
	debugDataPublisher.PublishIfActive(livedebugging.NewData(
		livedebugging.ComponentID(componentID),
		livedebugging.OtelLog,
		uint64(ld.LogRecordCount()),
		func() string {
			data, err := textmarshaler.MarshalLogs(ld)
			if err != nil {
				return ""
			}
			return string(data)
		},
		livedebugging.WithTargetComponentIDs(extractIds(nextLogs)),
	))
}

func PublishTracesIfActive(debugDataPublisher livedebugging.DebugDataPublisher, componentID string, td ptrace.Traces, nextTraces []otelcol.ComponentMetadata) {
	debugDataPublisher.PublishIfActive(livedebugging.NewData(
		livedebugging.ComponentID(componentID),
		livedebugging.OtelTrace,
		uint64(td.SpanCount()),
		func() string {
			data, err := textmarshaler.MarshalTraces(td)
			if err != nil {
				return ""
			}
			return string(data)
		},
		livedebugging.WithTargetComponentIDs(extractIds(nextTraces)),
	))
}

func PublishMetricsIfActive(debugDataPublisher livedebugging.DebugDataPublisher, componentID string, md pmetric.Metrics, nextMetrics []otelcol.ComponentMetadata) {
	debugDataPublisher.PublishIfActive(livedebugging.NewData(
		livedebugging.ComponentID(componentID),
		livedebugging.OtelMetric,
		uint64(md.MetricCount()),
		func() string {
			data, err := textmarshaler.MarshalMetrics(md)
			if err != nil {
				return ""
			}
			return string(data)
		},
		livedebugging.WithTargetComponentIDs(extractIds(nextMetrics)),
	))
}

func extractIds(consumers []otelcol.ComponentMetadata) []string {
	ids := make([]string, 0, len(consumers))
	for _, cons := range consumers {
		ids = append(ids, cons.ComponentID())
	}
	return ids
}
