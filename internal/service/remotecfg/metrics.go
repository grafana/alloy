package remotecfg

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	lastLoadSuccess        prometheus.Gauge
	lastFetchNotModified   prometheus.Gauge
	totalFailures          prometheus.Counter
	configHash             *prometheus.GaugeVec
	lastReceivedConfigHash *prometheus.GaugeVec
	lastFetchSuccessTime   prometheus.Gauge
	totalAttempts          prometheus.Counter
	getConfigTime          prometheus.Histogram
}

func registerMetrics(reg prometheus.Registerer) *metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	m := &metrics{
		configHash: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "remotecfg_hash",
				Help: "Hash of the currently active remote configuration.",
			},
			[]string{"hash"},
		),
		lastReceivedConfigHash: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_received_hash",
				Help: "Hash of the last received remote configuration.",
			},
			[]string{"hash"},
		),
		lastLoadSuccess: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_load_successful",
				Help: "Remote config loaded successfully",
			},
		),
		lastFetchNotModified: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_load_not_modified",
				Help: "Remote config not modified since last fetch",
			},
		),
		totalFailures: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "remotecfg_load_failures_total",
				Help: "Remote configuration load failures",
			},
		),
		totalAttempts: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "remotecfg_load_attempts_total",
				Help: "Attempts to load remote configuration",
			},
		),
		lastFetchSuccessTime: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "remotecfg_last_load_success_timestamp_seconds",
				Help: "Timestamp of the last successful remote configuration load",
			},
		),
		getConfigTime: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: "remotecfg_request_duration_seconds",
				Help: "Duration of remote configuration requests.",
			},
		),
	}

	// Register metrics safely - ignore AlreadyRegisteredError
	safeRegister(reg, m.configHash)
	safeRegister(reg, m.lastLoadSuccess)
	safeRegister(reg, m.lastFetchNotModified)
	safeRegister(reg, m.totalFailures)
	safeRegister(reg, m.totalAttempts)
	safeRegister(reg, m.lastFetchSuccessTime)
	safeRegister(reg, m.getConfigTime)

	return m
}

// safeRegister registers a metric with the registerer, ignoring AlreadyRegisteredError
func safeRegister(reg prometheus.Registerer, c prometheus.Collector) {
	err := reg.Register(c)
	if err != nil {
		var alreadyRegErr prometheus.AlreadyRegisteredError
		if !errors.As(err, &alreadyRegErr) {
			panic(err)
		}
		// If it is an AlreadyRegisteredError, we ignore it silently
	}
}
