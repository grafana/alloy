package ssh

import (
	"github.com/prometheus/client_golang/prometheus"
)

// DiskCollector collects disk-related metrics via SSH
type DiskCollector struct {
	Utilization         *prometheus.GaugeVec
	UsedBytes           *prometheus.GaugeVec
	AvailableBytes      *prometheus.GaugeVec
	ReadBytesTotal      *prometheus.GaugeVec
	WriteBytesTotal     *prometheus.GaugeVec
}

// NewDiskCollector initializes the disk collector
func NewDiskCollector() *DiskCollector {
	return &DiskCollector{
		Utilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_disk_utilization",
				Help: "Disk utilization percentage by mountpoint",
			},
			[]string{"device", "fstype", "mountpoint"},
		),
		UsedBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_disk_used_bytes",
				Help: "Disk used bytes by mountpoint",
			},
			[]string{"device", "fstype", "mountpoint"},
		),
		AvailableBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_disk_available_bytes",
				Help: "Disk available bytes by mountpoint",
			},
			[]string{"device", "fstype", "mountpoint"},
		),
		ReadBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_disk_read_bytes_total",
				Help: "Total bytes read from disk",
			},
			[]string{"device"},
		),
		WriteBytesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "ssh_disk_write_bytes_total",
				Help: "Total bytes written to disk",
			},
			[]string{"device"},
		),
	}
}

// Describe sends the descriptors of each metric to Prometheus
func (d *DiskCollector) Describe(ch chan<- *prometheus.Desc) {
	d.Utilization.Describe(ch)
	d.UsedBytes.Describe(ch)
	d.AvailableBytes.Describe(ch)
	d.ReadBytesTotal.Describe(ch)
	d.WriteBytesTotal.Describe(ch)
}

// Collect sends the metrics to Prometheus
func (d *DiskCollector) Collect(ch chan<- prometheus.Metric) {
	d.Utilization.Collect(ch)
	d.UsedBytes.Collect(ch)
	d.AvailableBytes.Collect(ch)
	d.ReadBytesTotal.Collect(ch)
	d.WriteBytesTotal.Collect(ch)
}
