package relabel

import (
	"github.com/grafana/alloy/internal/util"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	entriesProcessed prometheus_client.Counter
	entriesOutgoing  prometheus_client.Counter
	cacheHits        prometheus_client.Counter
	cacheMisses      prometheus_client.Counter
	cacheSize        prometheus_client.Gauge

	// Forward queue metrics
	droppedEntriesTotal prometheus_client.Counter
	enqueueRetriesTotal prometheus_client.Counter
}

// newMetrics creates a new set of metrics. If reg is non-nil, the metrics
// will also be registered.
func newMetrics(reg prometheus_client.Registerer) *metrics {
	var m metrics

	m.entriesProcessed = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_relabel_entries_processed",
		Help: "Total number of log entries processed",
	})
	m.entriesOutgoing = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_relabel_entries_written",
		Help: "Total number of log entries forwarded",
	})
	m.cacheMisses = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_relabel_cache_misses",
		Help: "Total number of cache misses",
	})
	m.cacheHits = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_relabel_cache_hits",
		Help: "Total number of cache hits",
	})
	m.cacheSize = prometheus_client.NewGauge(prometheus_client.GaugeOpts{
		Name: "loki_relabel_cache_size",
		Help: "Total size of relabel cache",
	})
	m.droppedEntriesTotal = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_relabel_dropped_entries_total",
		Help: "Total number of log entries dropped because the destination queue was full",
	})
	m.enqueueRetriesTotal = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "loki_relabel_enqueue_retries_total",
		Help: "Total number of times enqueueing was retried when block_on_full is enabled",
	})

	if reg != nil {
		m.entriesProcessed = util.MustRegisterOrGet(reg, m.entriesProcessed).(prometheus_client.Counter)
		m.entriesOutgoing = util.MustRegisterOrGet(reg, m.entriesOutgoing).(prometheus_client.Counter)
		m.cacheMisses = util.MustRegisterOrGet(reg, m.cacheMisses).(prometheus_client.Counter)
		m.cacheHits = util.MustRegisterOrGet(reg, m.cacheHits).(prometheus_client.Counter)
		m.cacheSize = util.MustRegisterOrGet(reg, m.cacheSize).(prometheus_client.Gauge)
		m.droppedEntriesTotal = util.MustRegisterOrGet(reg, m.droppedEntriesTotal).(prometheus_client.Counter)
		m.enqueueRetriesTotal = util.MustRegisterOrGet(reg, m.enqueueRetriesTotal).(prometheus_client.Counter)
	}

	return &m
}
