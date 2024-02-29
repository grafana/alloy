package vault

import "github.com/prometheus/client_golang/prometheus"

type metrics struct {
	authTotal       prometheus.Counter
	secretReadTotal prometheus.Counter

	authLeaseRenewalTotal   prometheus.Counter
	secretLeaseRenewalTotal prometheus.Counter
}

func newMetrics(r prometheus.Registerer) *metrics {
	var m metrics

	m.authTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "remote_vault_auth_total",
		Help: "Total number of times this component authenticated to Vault",
	})
	m.secretReadTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "remote_vault_secret_reads_total",
		Help: "Total number of times the secret was read from Vault",
	})

	m.authLeaseRenewalTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "remote_vault_auth_lease_renewal_total",
		Help: "Total number of times this component renewed its auth token lease",
	})
	m.secretLeaseRenewalTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "remote_vault_secret_lease_renewal_total",
		Help: "Total number of times this component renewed its secret lease",
	})

	if r != nil {
		r.MustRegister(
			m.authTotal,
			m.secretReadTotal,

			m.authLeaseRenewalTotal,
			m.secretLeaseRenewalTotal,
		)
	}
	return &m
}
