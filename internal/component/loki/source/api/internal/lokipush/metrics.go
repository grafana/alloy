package lokipush

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

func newMetircs(reg prometheus.Registerer) *metrics {
	m := &metrics{
		entriesWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "loki_source_api_entries_written",
			Help: "Total number of entries written.",
		}),
	}
	m.entriesWritten = util.MustRegisterOrGet(reg, m.entriesWritten).(prometheus.Counter)
	return m
}

type metrics struct {
	entriesWritten prometheus.Counter
}
