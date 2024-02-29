package convert

import (
	"github.com/prometheus/client_golang/prometheus"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	entriesTotal     prometheus_client.Counter
	entriesFailed    prometheus_client.Counter
	entriesProcessed prometheus_client.Counter
}

func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics

	m.entriesTotal = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "otelcol_exporter_loki_entries_total",
		Help: "Total number of log entries passed through the converter",
	})
	m.entriesFailed = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "otelcol_exporter_loki_entries_failed",
		Help: "Total number of log entries failed to convert",
	})
	m.entriesProcessed = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "otelcol_exporter_loki_entries_processed",
		Help: "Total number of log entries successfully converted",
	})

	if reg != nil {
		reg.MustRegister(
			m.entriesTotal,
			m.entriesFailed,
			m.entriesProcessed,
		)
	}

	return &m
}
