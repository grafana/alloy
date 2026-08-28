package heroku

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	entriesWritten prometheus.Counter
	parsingErrors  prometheus.Counter
}

func newMetrics(reg prometheus.Registerer) *metrics {
	return &metrics{
		entriesWritten: util.MustRegisterOrGet(reg, prometheus.NewCounter(prometheus.CounterOpts{
			Name: "loki_source_heroku_drain_entries_total",
			Help: "Number of successful entries received by the Heroku target",
		})).(prometheus.Counter),
		parsingErrors: util.MustRegisterOrGet(reg, prometheus.NewCounter(prometheus.CounterOpts{
			Name: "loki_source_heroku_drain_parsing_errors_total",
			Help: "Number of parsing errors while receiving Heroku messages",
		})).(prometheus.Counter),
	}
}
