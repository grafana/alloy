package ssh

import (
	"github.com/prometheus/client_golang/prometheus"
)

// CPUCollector collects CPU-related metrics via SSH
type CPUCollector struct {
	Utilization    prometheus.Gauge
	UserUtilization prometheus.Gauge
	SystemUtilization prometheus.Gauge
}

// NewCPUCollector initializes the CPU collector
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{
		Utilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_cpu_utilization",
			Help: "CPU utilization percentage",
		}),
		UserUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_cpu_user_utilization",
			Help: "CPU user utilization percentage",
		}),
		SystemUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_cpu_system_utilization",
			Help: "CPU system utilization percentage",
		}),
	}
}

// Describe sends the descriptors of each metric to Prometheus
func (c *CPUCollector) Describe(ch chan<- *prometheus.Desc) {
	c.Utilization.Describe(ch)
	c.UserUtilization.Describe(ch)
	c.SystemUtilization.Describe(ch)
}

// Collect sends the metrics to Prometheus
func (c *CPUCollector) Collect(ch chan<- prometheus.Metric) {
	c.Utilization.Collect(ch)
	c.UserUtilization.Collect(ch)
	c.SystemUtilization.Collect(ch)
}
