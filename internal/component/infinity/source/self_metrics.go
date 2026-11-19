package source

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

// selfMetrics use the label "query". Alloy's own scrape sets "instance", so
// that name would clash.
type selfMetrics struct {
	pollDuration    *prometheus.HistogramVec
	pollFailures    *prometheus.CounterVec
	pollsOverrun    *prometheus.CounterVec
	samplesSent     *prometheus.CounterVec
	entriesSent     *prometheus.CounterVec
	duplicateSeries *prometheus.CounterVec
}

func newSelfMetrics(reg prometheus.Registerer) *selfMetrics {
	m := &selfMetrics{
		pollDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "infinity_source_poll_duration_seconds",
			Help:    "Duration of one poll of a query.",
			Buckets: prometheus.DefBuckets,
		}, []string{"query"}),
		pollFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "infinity_source_poll_failures_total",
			Help: "Total number of failed polls, by reason.",
		}, []string{"query", "reason"}),
		pollsOverrun: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "infinity_source_polls_overrun_total",
			Help: "Total number of polls that took longer than the interval.",
		}, []string{"query"}),
		samplesSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "infinity_source_samples_sent_total",
			Help: "Total number of samples sent, including up.",
		}, []string{"query"}),
		entriesSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "infinity_source_entries_sent_total",
			Help: "Total number of log entries sent.",
		}, []string{"query"}),
		duplicateSeries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "infinity_source_duplicate_series_total",
			Help: "Total number of samples dropped because an earlier row had the same labels.",
		}, []string{"query"}),
	}
	// Alloy keeps the registry of a component across a failed build, so a
	// rebuild must use the collectors that are already there.
	m.pollDuration = util.MustRegisterOrGet(reg, m.pollDuration).(*prometheus.HistogramVec)
	m.pollFailures = util.MustRegisterOrGet(reg, m.pollFailures).(*prometheus.CounterVec)
	m.pollsOverrun = util.MustRegisterOrGet(reg, m.pollsOverrun).(*prometheus.CounterVec)
	m.samplesSent = util.MustRegisterOrGet(reg, m.samplesSent).(*prometheus.CounterVec)
	m.entriesSent = util.MustRegisterOrGet(reg, m.entriesSent).(*prometheus.CounterVec)
	m.duplicateSeries = util.MustRegisterOrGet(reg, m.duplicateSeries).(*prometheus.CounterVec)
	return m
}

func (m *selfMetrics) deleteQuery(query string) {
	l := prometheus.Labels{"query": query}
	m.pollDuration.DeletePartialMatch(l)
	m.pollFailures.DeletePartialMatch(l)
	m.pollsOverrun.DeletePartialMatch(l)
	m.samplesSent.DeletePartialMatch(l)
	m.entriesSent.DeletePartialMatch(l)
	m.duplicateSeries.DeletePartialMatch(l)
}
