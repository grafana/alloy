package ssh

import (
	"github.com/prometheus/client_golang/prometheus"
)

// MemoryCollector collects memory-related metrics via SSH
type MemoryCollector struct {
	Utilization          prometheus.Gauge
	SwapUtilization      prometheus.Gauge
	AvailableMemoryBytes prometheus.Gauge
	AvailableSwapBytes   prometheus.Gauge
}

// NewMemoryCollector initializes the memory collector
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{
		Utilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_memory_utilization",
			Help: "Memory utilization percentage",
		}),
		SwapUtilization: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_memory_swap_utilization",
			Help: "Swap memory utilization percentage",
		}),
		AvailableMemoryBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_memory_available_bytes",
			Help: "Available memory in bytes",
		}),
		AvailableSwapBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ssh_memory_available_swap_bytes",
			Help: "Available swap memory in bytes",
		}),
	}
}

// Describe sends the descriptors of each metric to Prometheus
func (m *MemoryCollector) Describe(ch chan<- *prometheus.Desc) {
	m.Utilization.Describe(ch)
	m.SwapUtilization.Describe(ch)
	m.AvailableMemoryBytes.Describe(ch)
	m.AvailableSwapBytes.Describe(ch)
}

// Collect sends the metrics to Prometheus
func (m *MemoryCollector) Collect(ch chan<- prometheus.Metric) {
	m.Utilization.Collect(ch)
	m.SwapUtilization.Collect(ch)
	m.AvailableMemoryBytes.Collect(ch)
	m.AvailableSwapBytes.Collect(ch)
}
