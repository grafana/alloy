package secrets_manager

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	fetchesTotal *prometheus.CounterVec
	lastAccessed prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		fetchesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "remote_aws_secrets_manager_fetches_total",
			Help: "Total number of secret fetches from AWS Secrets Manager, by result.",
		}, []string{"result"}),
		lastAccessed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_aws_secrets_manager_timestamp_last_accessed_unix_seconds",
			Help: "The last successful access in Unix seconds.",
		}),
	}
	// The runtime keeps one registry per component across failed builds.
	// Reuse the collectors that an earlier build registered.
	m.fetchesTotal = util.MustRegisterOrGet(reg, m.fetchesTotal).(*prometheus.CounterVec)
	m.lastAccessed = util.MustRegisterOrGet(reg, m.lastAccessed).(prometheus.Gauge)
	// Make both series now. An alert on the error rate then has data before the first failure.
	m.fetchesTotal.WithLabelValues("success")
	m.fetchesTotal.WithLabelValues("error")
	return m
}
