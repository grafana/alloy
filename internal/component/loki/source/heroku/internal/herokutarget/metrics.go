package herokutarget

// This code is copied from Promtail. The herokutarget package is used to
// configure and run the targets that can read heroku entries and forward them
// to other loki components.

import "github.com/prometheus/client_golang/prometheus"

type Metrics struct {
	herokuEntries prometheus.Counter
	herokuErrors  prometheus.Counter
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics

	m.herokuEntries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_heroku_drain_entries_total",
		Help: "Number of successful entries received by the Heroku target",
	})

	m.herokuErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_heroku_drain_parsing_errors_total",
		Help: "Number of parsing errors while receiving Heroku messages",
	})

	reg.MustRegister(m.herokuEntries, m.herokuErrors)
	return &m
}
