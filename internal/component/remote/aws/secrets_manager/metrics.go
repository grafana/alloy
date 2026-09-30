package secrets_manager

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	fetchesTotal *prometheus.CounterVec
	lastSuccess  prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
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
	for _, c := range []prometheus.Collector{m.fetchesTotal, m.lastSuccess} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}
