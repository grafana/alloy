package target

// This code is copied from Promtail (https://github.com/grafana/loki/commit/954df433e98f659d006ced52b23151cb5eb2fdfa) with minor edits. The target package is used to
// configure and run the targets that can read journal entries and forward them
// to other loki components.

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds a set of journal target metrics.
type Metrics struct {
	reg prometheus.Registerer

	journalErrors *prometheus.CounterVec
	journalLines  prometheus.Counter
}

// NewMetrics creates a new set of journal target metrics. If reg is non-nil, the
// metrics will be registered.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics
	m.reg = reg

	m.journalErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_journal_target_parsing_errors_total",
		Help: "Total number of parsing errors while reading journal messages",
	}, []string{"error"})
	m.journalLines = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_journal_target_lines_total",
		Help: "Total number of successful journal lines read",
	})

	if reg != nil {
		reg.MustRegister(
			m.journalErrors,
			m.journalLines,
		)
	}

	return &m
}
