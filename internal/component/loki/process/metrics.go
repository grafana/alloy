package process

import (
	"github.com/grafana/alloy/internal/util"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

// forwardMetrics contains metrics related to log entry forwarding.
type forwardMetrics struct {
	// Number of log entries dropped because the destination queue was full.
	droppedEntriesTotal prometheus_client.Counter

	// Number of times we retried enqueueing when block_on_full is enabled.
	enqueueRetriesTotal prometheus_client.Counter
}

// newForwardMetrics creates a new set of forward metrics. If reg is non-nil,
// the metrics will also be registered.
func newForwardMetrics(reg prometheus_client.Registerer) *forwardMetrics {
	var m forwardMetrics

	m.droppedEntriesTotal = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_process_dropped_entries_total",
		Help: "Total number of log entries dropped because the destination queue was full",
	})

	m.enqueueRetriesTotal = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_process_enqueue_retries_total",
		Help: "Total number of times enqueueing was retried when block_on_full is enabled",
	})

	if reg != nil {
		m.droppedEntriesTotal = util.MustRegisterOrGet(reg, m.droppedEntriesTotal).(prometheus_client.Counter)
		m.enqueueRetriesTotal = util.MustRegisterOrGet(reg, m.enqueueRetriesTotal).(prometheus_client.Counter)
	}

	return &m
}
