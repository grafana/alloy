package secrets_manager

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

type metrics struct {
	fetchesTotal *prometheus.CounterVec
	lastSuccess  prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		fetchesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "remote_aws_secrets_manager_fetches_total",
			Help: "Total number of secret fetches from AWS Secrets Manager, by result.",
		}, []string{"result"}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remote_aws_secrets_manager_timestamp_last_success_unix_seconds",
			Help: "Time of the last successful secret fetch, in Unix seconds.",
		}),
	}
	// The runtime keeps one registry per component across failed builds.
	// Reuse the collectors that an earlier build registered.
	m.fetchesTotal = util.MustRegisterOrGet(reg, m.fetchesTotal).(*prometheus.CounterVec)
	m.lastSuccess = util.MustRegisterOrGet(reg, m.lastSuccess).(prometheus.Gauge)
	return m
}
