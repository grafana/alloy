//go:build linux && cgo && promtail_journal_enabled

package journal

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

const (
	noMessageError   = "no_message"
	emptyLabelsError = "empty_labels"
)

// metrics holds a set of journal target metrics.
type metrics struct {
	reg prometheus.Registerer

	journalErrors *prometheus.CounterVec
	journalLines  prometheus.Counter
}

// newMetrics creates a new set of journal target metrics. If reg is non-nil, the
// metrics will be registered.
func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics
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
		m.journalErrors = util.MustRegisterOrGet(reg, m.journalErrors).(*prometheus.CounterVec)
		m.journalLines = util.MustRegisterOrGet(reg, m.journalLines).(prometheus.Counter)
	}

	return &m
}
