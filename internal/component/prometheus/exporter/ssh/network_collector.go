package ssh

import (
	"github.com/prometheus/client_golang/prometheus"
)

// NetworkCollector collects network-related metrics via SSH
type NetworkCollector struct {
	ReceiveBytesTotal *prometheus.GaugeVec
	TransmitBytesTotal *prometheus.GaugeVec
}

// NewNetworkCollector initializes the network collector
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{
		ReceiveBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_network_receive_bytes_total",
				Help: "Total bytes received on the network interface",
			},
			[]string{"interface"},
		),
		TransmitBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_network_transmit_bytes_total",
				Help: "Total bytes transmitted on the network interface",
			},
			[]string{"interface"},
		),
	}
}

// Describe sends the descriptors of each metric to Prometheus
func (n *NetworkCollector) Describe(ch chan<- *prometheus.Desc) {
	n.ReceiveBytesTotal.Describe(ch)
	n.TransmitBytesTotal.Describe(ch)
}

// Collect sends the metrics to Prometheus
func (n *NetworkCollector) Collect(ch chan<- prometheus.Metric) {
	n.ReceiveBytesTotal.Collect(ch)
	n.TransmitBytesTotal.Collect(ch)
}
