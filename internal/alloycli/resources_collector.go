package alloycli

import (
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/service/logging/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// resourcesCollector is a prometheus.Collector which exposes process-level
// statistics. It is similar to the process collector in
// github.com/prometheus/client_golang but includes support for more platforms.
type resourcesCollector struct {
	log log.Logger

	processStartTime *prometheus.Desc
	cpuTotal         *prometheus.Desc
	rssMemory        *prometheus.Desc
	virtMemory       *prometheus.Desc
	rxBytes          *prometheus.Desc
	txBytes          *prometheus.Desc
}

var _ prometheus.Collector = (*resourcesCollector)(nil)

// newResourcesCollector creates a new resourcesCollector.
func newResourcesCollector(l log.Logger) *resourcesCollector {
	rc := &resourcesCollector{
		log: l,

		processStartTime: prometheus.NewDesc(
			"alloy_resources_process_start_time_seconds",
			"Start time of the process since Unix epoch in seconds.",
			nil, nil,
		),

		cpuTotal: prometheus.NewDesc(
			"alloy_resources_process_cpu_seconds_total",
			"Total user and system CPU time spent in seconds.",
			nil, nil,
		),

		rssMemory: prometheus.NewDesc(
			"alloy_resources_process_resident_memory_bytes",
			"Current resident memory size in bytes.",
			nil, nil,
		),

		virtMemory: prometheus.NewDesc(
			"alloy_resources_process_virtual_memory_bytes",
			"Current virtual memory size in bytes.",
			nil, nil,
		),

		rxBytes: prometheus.NewDesc(
			"alloy_resources_machine_rx_bytes_total",
			"Total bytes, host-wide, received across all network interfaces.",
			nil, nil,
		),

		txBytes: prometheus.NewDesc(
			"alloy_resources_machine_tx_bytes_total",
			"Total bytes, host-wide, sent across all given network interface.",
			nil, nil,
		),
	}

	return rc
}

func (rc *resourcesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.processStartTime
	ch <- rc.cpuTotal
	ch <- rc.rssMemory
	ch <- rc.virtMemory
	ch <- rc.rxBytes
	ch <- rc.txBytes
}

func (rc *resourcesCollector) Collect(ch chan<- prometheus.Metric) {
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		level.Error(rc.log).Log("msg", "failed to get process", "err", err)
		return
	}

	if t, err := proc.CreateTime(); err != nil {
		rc.reportError(rc.processStartTime, err)
	} else {
		dur := time.Duration(t) * time.Millisecond

		ch <- prometheus.MustNewConstMetric(
			rc.processStartTime,
			prometheus.GaugeValue,
			dur.Seconds(),
		)
	}

	if ts, err := proc.Times(); err != nil {
		rc.reportError(rc.cpuTotal, err)
	} else {
		ch <- prometheus.MustNewConstMetric(
			rc.cpuTotal,
			prometheus.CounterValue,
			ts.User+ts.System,
		)
	}

	if mi, err := proc.MemoryInfo(); err != nil {
		rc.reportError(rc.virtMemory, err)
		rc.reportError(rc.rssMemory, err)
	} else {
		ch <- prometheus.MustNewConstMetric(
			rc.virtMemory,
			prometheus.GaugeValue,
			float64(mi.VMS),
		)

		ch <- prometheus.MustNewConstMetric(
			rc.rssMemory,
			prometheus.GaugeValue,
			float64(mi.RSS),
		)
	}

	if counters, err := net.IOCounters(true); err != nil {
		rc.reportError(rc.rxBytes, err)
		rc.reportError(rc.txBytes, err)
	} else {
		var rxBytes, txByes uint64

		for _, counter := range counters {
			rxBytes += counter.BytesRecv
			txByes += counter.BytesSent
		}

		ch <- prometheus.MustNewConstMetric(
			rc.rxBytes,
			prometheus.CounterValue,
			float64(rxBytes),
		)

		ch <- prometheus.MustNewConstMetric(
			rc.txBytes,
			prometheus.CounterValue,
			float64(txByes),
		)
	}
}

func (rc *resourcesCollector) reportError(d *prometheus.Desc, err error) {
	level.Error(rc.log).Log("msg", "failed to collect resources metric", "name", d.String(), "err", err)
}
