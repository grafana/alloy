package relabel

import (
	"github.com/prometheus/client_golang/prometheus"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	entriesProcessed prometheus_client.Counter
	entriesOutgoing  prometheus_client.Counter
	cacheHits        prometheus_client.Counter
	cacheMisses      prometheus_client.Counter
	cacheSize        prometheus_client.Gauge
}

// newMetrics creates a new set of metrics. If reg is non-nil, the metrics
// will also be registered.
func newMetrics(reg prometheus.Registerer) *metrics {
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

	if reg != nil {
		reg.MustRegister(
			m.entriesProcessed,
			m.entriesOutgoing,
			m.cacheMisses,
			m.cacheHits,
			m.cacheSize,
		)
	}

	return &m
}
