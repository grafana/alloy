package receive_http

import (
	pyrometricsutil "github.com/grafana/alloy/internal/component/pyroscope/util/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	debugInfoDownstreamCalls *prometheus.CounterVec
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		debugInfoDownstreamCalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pyroscope_receive_http_debuginfo_downstream_calls_total",
			Help: "Total number of downstream debuginfo calls made by the receive_http proxy, labeled by method and result.",
		}, []string{"method", "result"}),
	}

	if reg != nil {
		m.debugInfoDownstreamCalls = pyrometricsutil.MustRegisterOrGet(reg, m.debugInfoDownstreamCalls).(*prometheus.CounterVec)
	}

	return m
}
