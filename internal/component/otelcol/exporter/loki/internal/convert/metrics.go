package convert

import (
	"github.com/grafana/alloy/internal/util"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	entriesTotal     prometheus_client.Counter
	entriesFailed    prometheus_client.Counter
	entriesProcessed prometheus_client.Counter
}

func newMetrics(reg prometheus_client.Registerer) *metrics {
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
		m.entriesTotal = util.MustRegisterOrGet(reg, m.entriesTotal).(prometheus_client.Counter)
		m.entriesFailed = util.MustRegisterOrGet(reg, m.entriesFailed).(prometheus_client.Counter)
		m.entriesProcessed = util.MustRegisterOrGet(reg, m.entriesProcessed).(prometheus_client.Counter)
	}

	return &m
}
