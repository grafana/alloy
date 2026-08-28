package instrumentation

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type remoteConfigMetrics struct {
	fetchStatusCodes   *prometheus.CounterVec
	fetchErrors        prometheus.Counter
	invalidConfigFetch *prometheus.CounterVec
}

var remoteConfMetrics *remoteConfigMetrics
var remoteConfMetricsInitializer sync.Once

func initializeRemoteConfigMetrics() {
	remoteConfMetrics = newRemoteConfigMetrics()
}

func newRemoteConfigMetrics() *remoteConfigMetrics {
	var remoteConfigMetrics remoteConfigMetrics

	remoteConfigMetrics.fetchStatusCodes = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_remote_config_fetches_total",
			Help: "Number of fetch requests for the remote config by HTTP status code",
		},
		[]string{"status_code"},
	)
	remoteConfigMetrics.fetchErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "agent_remote_config_fetch_errors_total",
			Help: "Number of errors attempting to fetch remote config",
		},
	)

	remoteConfigMetrics.invalidConfigFetch = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_remote_config_invalid_total",
			Help: "Number of validation errors by reason (i.e. invalid yaml or an invalid agent config)",
		},
		[]string{"reason"},
	)

	return &remoteConfigMetrics
}

func InstrumentRemoteConfigFetch(statusCode int) {
	remoteConfMetricsInitializer.Do(initializeRemoteConfigMetrics)
	remoteConfMetrics.fetchStatusCodes.WithLabelValues(fmt.Sprintf("%d", statusCode)).Inc()
}

func InstrumentRemoteConfigFetchError() {
	remoteConfMetricsInitializer.Do(initializeRemoteConfigMetrics)
	remoteConfMetrics.fetchErrors.Inc()
}

func InstrumentInvalidRemoteConfig(reason string) {
	remoteConfMetricsInitializer.Do(initializeRemoteConfigMetrics)
	remoteConfMetrics.invalidConfigFetch.WithLabelValues(reason).Inc()
}
