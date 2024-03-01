package target

// This code is copied from Promtail. The target package is used to
// configure and run the targets that can read gelf entries and forward them
// to other loki components.

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds a set of gelf metrics.
type Metrics struct {
	reg prometheus.Registerer

	gelfEntries prometheus.Counter
	gelfErrors  prometheus.Counter
}

// NewMetrics creates a new set of gelf metrics. If reg is non-nil, the
// metrics will be registered.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics
	m.reg = reg

	m.gelfEntries = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "agent",
		Name:      "loki_source_gelf_target_entries_total",
		Help:      "Total number of successful entries sent to the gelf target",
	})
	m.gelfErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "agent",
		Name:      "loki_source_gelf_target_parsing_errors_total",
		Help:      "Total number of parsing errors while receiving gelf messages",
	})

	if reg != nil {
		reg.MustRegister(
			m.gelfEntries,
			m.gelfErrors,
		)
	}

	return &m
}
