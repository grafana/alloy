package syslogtarget

// This code is copied from Promtail. The syslogtarget package is used to
// configure and run the targets that can read syslog entries and forward them
// to other loki components.

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds a set of syslog metrics.
type Metrics struct {
	reg prometheus.Registerer

	syslogEntries       prometheus.Counter
	syslogParsingErrors prometheus.Counter
	syslogEmptyMessages prometheus.Counter
}

// NewMetrics creates a new set of syslog metrics. If reg is non-nil, the
// metrics will be registered.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics
	m.reg = reg

	m.syslogEntries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_syslog_entries_total",
		Help: "Total number of successful entries sent to the syslog target",
	})
	m.syslogParsingErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_syslog_parsing_errors_total",
		Help: "Total number of parsing errors while receiving syslog messages",
	})
	m.syslogEmptyMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_syslog_empty_messages_total",
		Help: "Total number of empty messages received from syslog",
	})

	if reg != nil {
		reg.MustRegister(
			m.syslogEntries,
			m.syslogParsingErrors,
			m.syslogEmptyMessages,
		)
	}

	return &m
}
