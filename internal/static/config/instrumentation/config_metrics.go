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

func initializeConfigMetrics(withClusterLabel bool) {
	confMetrics = newConfigMetrics(withClusterLabel)
}

func newConfigMetrics(withClusterLabel bool) *configMetrics {
	var m configMetrics
	var labels []string
	if withClusterLabel {
		labels = []string{"sha256", "alloy_cluster"}
	} else {
		labels = []string{"sha256"}
	}

	m.configHash = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alloy_config_hash",
			Help: "Hash of the currently active config file.",
		},
		labels,
	)
	m.configLoadSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alloy_config_last_load_successful",
		Help: "Config loaded successfully.",
	})
	m.configLoadSuccessSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alloy_config_last_load_success_timestamp_seconds",
		Help: "Timestamp of the last successful configuration load.",
	})
	m.configLoadFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "alloy_config_load_failures_total",
		Help: "Configuration load failures.",
	})
	return &m
}

func InstrumentConfig(success bool, hash [sha256.Size]byte, clusterName string) {
	configMetricsInitializer.Do(func() {
		initializeConfigMetrics(clusterName != "")
	})

	if success {
		confMetrics.configLoadSuccessSeconds.SetToCurrentTime()
		confMetrics.configLoadSuccess.Set(1)
	} else {
		confMetrics.configLoadSuccess.Set(0)
		confMetrics.configLoadFailures.Inc()
	}

	confMetrics.configHash.Reset()
	if clusterName != "" {
		confMetrics.configHash.WithLabelValues(fmt.Sprintf("%x", hash), clusterName).Set(1)
	} else {
		confMetrics.configHash.WithLabelValues(fmt.Sprintf("%x", hash)).Set(1)
	}
}
