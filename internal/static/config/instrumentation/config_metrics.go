package instrumentation

import (
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// configMetrics exposes metrics related to configuration loading
type configMetrics struct {
	configHash               *prometheus.GaugeVec
	configLoadSuccess        prometheus.Gauge
	configLoadSuccessSeconds prometheus.Gauge
	configLoadFailures       prometheus.Counter
}

var confMetrics *configMetrics
var configMetricsInitializer sync.Once

func initializeConfigMetrics() {
	confMetrics = newConfigMetrics()
}

func newConfigMetrics() *configMetrics {
	var m configMetrics

	m.configHash = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agent_config_hash",
			Help: "Hash of the currently active config file.",
		},
		[]string{"sha256"},
	)
	m.configLoadSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "agent_config_last_load_successful",
		Help: "Config loaded successfully.",
	})
	m.configLoadSuccessSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "agent_config_last_load_success_timestamp_seconds",
		Help: "Timestamp of the last successful configuration load.",
	})
	m.configLoadFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "agent_config_load_failures_total",
		Help: "Configuration load failures.",
	})
	return &m
}

// Create a sha256 hash of the config before expansion and expose it via
// the agent_config_hash metric.
func InstrumentConfig(buf []byte) {
	InstrumentSHA256(sha256.Sum256(buf))
}

// InstrumentSHA256 stores the provided hash to the agent_config_hash metric.
func InstrumentSHA256(hash [sha256.Size]byte) {
	configMetricsInitializer.Do(initializeConfigMetrics)
	confMetrics.configHash.Reset()
	confMetrics.configHash.WithLabelValues(fmt.Sprintf("%x", hash)).Set(1)
}

// Expose metrics for load success / failures.
func InstrumentLoad(success bool) {
	configMetricsInitializer.Do(initializeConfigMetrics)
	if success {
		confMetrics.configLoadSuccessSeconds.SetToCurrentTime()
		confMetrics.configLoadSuccess.Set(1)
	} else {
		confMetrics.configLoadSuccess.Set(0)
		confMetrics.configLoadFailures.Inc()
	}
}
