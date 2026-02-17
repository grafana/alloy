package gelf

// This code is copied from Promtail. The target package is used to
// configure and run the targets that can read gelf entries and forward them
// to other loki components.

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

// metrics holds a set of gelf metrics.
type metrics struct {
	reg prometheus.Registerer

	entries prometheus.Counter
	errors  prometheus.Counter
}

// NewMetrics creates a new set of gelf metrics. If reg is non-nil, the
// metrics will be registered.
func NewMetrics(reg prometheus.Registerer) *metrics {
	var m metrics
	m.reg = reg

	m.entries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_gelf_target_entries_total",
		Help: "Total number of successful entries sent to the gelf target",
	})
	m.errors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_gelf_target_parsing_errors_total",
		Help: "Total number of parsing errors while receiving gelf messages",
	})

	if reg != nil {
		m.entries = util.MustRegisterOrGet(reg, m.entries).(prometheus.Counter)
		m.errors = util.MustRegisterOrGet(reg, m.errors).(prometheus.Counter)
	}

	return &m
}
