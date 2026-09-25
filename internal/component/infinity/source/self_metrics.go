package source

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
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

func newSelfMetrics(reg prometheus.Registerer) (*selfMetrics, error) {
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
	var errs []error
	for _, c := range []prometheus.Collector{m.pollDuration, m.pollFailures, m.pollsOverrun, m.samplesSent, m.entriesSent, m.duplicateSeries} {
		if err := reg.Register(c); err != nil {
			errs = append(errs, err)
		}
	}
	return m, errors.Join(errs...)
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
