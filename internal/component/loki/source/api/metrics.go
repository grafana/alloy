package api

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	entriesWritten prometheus.Counter
}

func newMetrics(reg prometheus.Registerer) *metrics {
	return &metrics{
		entriesWritten: util.MustRegisterOrGet(reg, prometheus.NewCounter(prometheus.CounterOpts{
			Name: "loki_source_api_entries_written",
			Help: "Total number of entries written.",
		})).(prometheus.Counter),
	}
}
