package docker

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/util"
)

// metrics holds a set of Docker target metrics.
type metrics struct {
	reg prometheus.Registerer

	dockerEntries prometheus.Counter
	dockerErrors  prometheus.Counter
}

// newMetrics creates a new set of Docker target metrics. If reg is non-nil, the
// metrics will be registered.
func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics
	m.reg = reg

	m.dockerEntries = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_docker_target_entries_total",
		Help: "Total number of successful entries sent to the Docker target",
	})
	m.dockerErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "loki_source_docker_target_parsing_errors_total",
		Help: "Total number of parsing errors while receiving Docker messages",
	})

	if reg != nil {
		m.dockerEntries = util.MustRegisterOrGet(reg, m.dockerEntries).(prometheus.Counter)
		m.dockerErrors = util.MustRegisterOrGet(reg, m.dockerErrors).(prometheus.Counter)
	}

	return &m
}
