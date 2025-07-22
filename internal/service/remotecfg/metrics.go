package remotecfg

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type metrics struct {
	lastLoadSuccess      prometheus.Gauge
	lastFetchNotModified prometheus.Gauge
	totalFailures        prometheus.Counter
	configHash           *prometheus.GaugeVec
	lastFetchSuccessTime prometheus.Gauge
	totalAttempts        prometheus.Counter
	getConfigTime        prometheus.Histogram
}

func registerMetrics(reg prometheus.Registerer) *metrics {
	prom := promauto.With(reg)
	return &metrics{
		configHash: prom.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "remotecfg_hash",
				Help: "Hash of the currently active remote configuration.",
			},
			[]string{"hash"},
		),
		lastLoadSuccess: prom.NewGauge(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_load_successful",
				Help: "Remote config loaded successfully",
			},
		),
		lastFetchNotModified: prom.NewGauge(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_load_not_modified",
				Help: "Remote config not modified since last fetch",
			},
		),
		totalFailures: prom.NewCounter(
			prometheus.CounterOpts{
				Name: "remotecfg_load_failures_total",
				Help: "Remote configuration load failures",
			},
		),
		totalAttempts: prom.NewCounter(
			prometheus.CounterOpts{
				Name: "remotecfg_load_attempts_total",
				Help: "Attempts to load remote configuration",
			},
		),
		lastFetchSuccessTime: prom.NewGauge(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_load_success_timestamp_seconds",
				Help: "Timestamp of the last successful remote configuration load",
			},
		),
		getConfigTime: prom.NewHistogram(
			prometheus.HistogramOpts{
				Name: "remotecfg_request_duration_seconds",
				Help: "Duration of remote configuration requests.",
			},
		),
	}
}
