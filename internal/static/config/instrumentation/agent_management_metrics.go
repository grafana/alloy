package instrumentation

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type agentManagementMetrics struct {
	configFallbacks *prometheus.CounterVec
}

var amMetrics *agentManagementMetrics
var amMetricsInitializer sync.Once

func initializeAgentManagementMetrics() {
	amMetrics = newAgentManagementMetrics()
}

func newAgentManagementMetrics() *agentManagementMetrics {
	var agentManagementMetrics agentManagementMetrics

	agentManagementMetrics.configFallbacks = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agent_management_config_fallbacks_total",
			Help: "Number of config fallbacks by fallback source.",
		},
		[]string{"fallback_to"},
	)

	return &agentManagementMetrics
}

func InstrumentAgentManagementConfigFallback(fallbackTo string) {
	amMetricsInitializer.Do(initializeAgentManagementMetrics)
	amMetrics.configFallbacks.WithLabelValues(fallbackTo).Inc()
}
