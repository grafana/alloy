package file

// This code is copied from loki/promtail@a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// The metrics struct provides a common set of metrics that are reused between all
// implementations of the reader interface.

import (
	"sync"

	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

// metrics hold the set of file-based metrics.
type metrics struct {
	// Registerer used. May be nil.
	reg prometheus.Registerer

	// File-specific metrics
	readBytes   *prometheus.GaugeVec
	totalBytes  *prometheus.GaugeVec
	readLines   *prometheus.CounterVec
	filesActive prometheus.Gauge

	// Reconciliation starts a replacement before the retired source has stopped. Several label sets can also read
	// the same path. Keep path-series until the last successful reader stops, including across a retry of a source.
	activePathsMu sync.Mutex
	activePaths   map[string]int
}

// newMetrics creates a new set of file metrics. If reg is non-nil, the metrics
// will be registered.
func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics
	m.reg = reg
	m.activePaths = make(map[string]int)

	m.readBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "loki_source_file_read_bytes_total",
		Help: "Number of bytes read.",
	}, []string{"path"})
	m.totalBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "loki_source_file_file_bytes_total",
		Help: "Number of bytes total.",
	}, []string{"path"})
	m.readLines = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_file_read_lines_total",
		Help: "Number of lines read.",
	}, []string{"path"})
	m.filesActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "loki_source_file_files_active_total",
		Help: "Number of active files.",
	})

	if reg != nil {
		m.readBytes = util.MustRegisterOrGet(reg, m.readBytes).(*prometheus.GaugeVec)
		m.totalBytes = util.MustRegisterOrGet(reg, m.totalBytes).(*prometheus.GaugeVec)
		m.readLines = util.MustRegisterOrGet(reg, m.readLines).(*prometheus.CounterVec)
		m.filesActive = util.MustRegisterOrGet(reg, m.filesActive).(prometheus.Gauge)
	}

	return &m
}

// startFileMetrics publishes a reader only after its file has been opened. An idle file needs a path-series on every
// successful run: a previous run's cleanup removes it, and a retry does not call the component's source factory again.
func (m *metrics) startFileMetrics(path string, size int64) {
	m.activePathsMu.Lock()
	defer m.activePathsMu.Unlock()
	m.activePaths[path]++
	m.totalBytes.WithLabelValues(path).Set(float64(size))
}

func (m *metrics) stopFileMetrics(path string) {
	m.activePathsMu.Lock()
	defer m.activePathsMu.Unlock()
	m.activePaths[path]--
	if m.activePaths[path] > 0 {
		return
	}
	delete(m.activePaths, path)
	m.readLines.DeleteLabelValues(path)
	m.readBytes.DeleteLabelValues(path)
	m.totalBytes.DeleteLabelValues(path)
}
