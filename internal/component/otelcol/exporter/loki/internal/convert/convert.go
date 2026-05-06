// Package convert implements conversion utilities to convert between
// OpenTelemetry Collector and Loki data.
//
// It follows the [OpenTelemetry Logs Data Model] and the [loki translator]
// package for implementing the conversion.
//
// [OpenTelemetry Logs Data Model]: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md
// [loki translator]: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/translator/loki
package convert

import (
	"context"
	"log/slog"

	loki_translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
)

// Converter implements consumer.Logs and converts received OTel logs into
// Loki-compatible log entries.
type Converter struct {
	log     *slog.Logger
	metrics *metrics

	next *loki.FanoutConsumer // Location to write converted logs.
}

var _ consumer.Logs = (*Converter)(nil)

// New returns a new Converter. Converted logs are passed to next.
func New(l *slog.Logger, r prometheus.Registerer, next *loki.FanoutConsumer) *Converter {
	if l == nil {
		l = logging.NewSlogNop()
	}
	m := newMetrics(r)
	return &Converter{log: l, metrics: m, next: next}
}

// Capabilities implements consumer.Logs.
func (conv *Converter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

// ConsumeLogs converts the provided OpenTelemetry Collector-formatted logs
// into Loki-compatible entries. Each call to ConsumeLogs will forward
// converted entries to `next`.
// This is reusing the logic from the OpenTelemetry Collector "contrib"
// distribution and its LogsToLokiRequests function.
func (conv *Converter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var entries []loki.Entry

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			scope := ills.At(j).Scope()
			for k := 0; k < logs.Len(); k++ {
				conv.metrics.entriesTotal.Inc()

				// TODO: loki added a parameter `defaultLabelsEnabled` to this function to add the possibility to disable default labels (exporter, job, instance, level)
				// Is this interesting for us in any ways? (@wildum)
				// https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/23863/files#diff-ef7831fcba373f6e8aa7f799b5b89f4e113b2064cd7ef1688286ce193d2256a8
				entry, err := loki_translator.LogToLokiEntry(logs.At(k), rls.At(i).Resource(), scope, nil)
				if err != nil {
					conv.log.Error("failed to convert log to loki entry", "err", err)
					conv.metrics.entriesFailed.Inc()
					continue
				}

				conv.metrics.entriesProcessed.Inc()
				entries = append(entries, loki.Entry{
					Labels: entry.Labels,
					Entry:  *entry.Entry,
				})
			}
		}
	}

	for _, entry := range entries {
		// NOTE: For now we stop on first error. Once we change to support batching we will batch
		// all converted logs and send it once. See https://github.com/grafana/alloy/issues/4953
		if err := conv.next.ConsumeEntry(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// UpdateFanout sets the locations the converter forwards log entries to.
func (conv *Converter) UpdateFanout(consumers []loki.Consumer) {
	conv.next.Update(consumers)
}
