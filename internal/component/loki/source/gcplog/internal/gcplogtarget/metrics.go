package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import "github.com/prometheus/client_golang/prometheus"

// Metrics stores gcplog entry metrics.
type Metrics struct {
	// reg is the Registerer used to create this set of metrics.
	reg prometheus.Registerer

	gcplogEntries                 *prometheus.CounterVec
	gcplogErrors                  *prometheus.CounterVec
	gcplogTargetLastSuccessScrape *prometheus.GaugeVec

	gcpPushEntries *prometheus.CounterVec
	gcpPushErrors  *prometheus.CounterVec
}

// NewMetrics creates a new set of metrics. Metrics will be registered to reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	var m Metrics
	m.reg = reg

	// Pull subscription metrics
	m.gcplogEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_gcplog_pull_entries_total",
		Help: "Number of entries received by the gcplog target",
	}, []string{"project"})

	m.gcplogErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_gcplog_pull_parsing_errors_total",
		Help: "Total number of parsing errors while receiving gcplog messages",
	}, []string{"project"})

	m.gcplogTargetLastSuccessScrape = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "loki_source_gcplog_pull_last_success_scrape",
		Help: "Timestamp of target's last successful poll",
	}, []string{"project", "target"})

	// Push subscription metrics
	m.gcpPushEntries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_gcplog_push_entries_total",
		Help: "Number of entries received by the gcplog target",
	}, []string{})

	m.gcpPushErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_gcplog_push_parsing_errors_total",
		Help: "Number of parsing errors while receiving gcplog messages",
	}, []string{"reason"})

	reg.MustRegister(
		m.gcplogEntries,
		m.gcplogErrors,
		m.gcplogTargetLastSuccessScrape,
		m.gcpPushEntries,
		m.gcpPushErrors,
	)
	return &m
}
