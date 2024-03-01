package file

// This code is copied from loki/promtail@a8d5815510bd959a6dd8c176a5d9fd9bbfc8f8b5.
// The metrics struct provides a common set of metrics that are reused between all
// implementations of the reader interface.

import "github.com/prometheus/client_golang/prometheus"

// metrics hold the set of file-based metrics.
type metrics struct {
	// Registerer used. May be nil.
	reg prometheus.Registerer

	// File-specific metrics
	readBytes        *prometheus.GaugeVec
	totalBytes       *prometheus.GaugeVec
	readLines        *prometheus.CounterVec
	encodingFailures *prometheus.CounterVec
	filesActive      prometheus.Gauge
}

// newMetrics creates a new set of file metrics. If reg is non-nil, the metrics
// will be registered.
func newMetrics(reg prometheus.Registerer) *metrics {
	var m metrics
	m.reg = reg

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
	m.encodingFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "loki_source_file_encoding_failures_total",
		Help: "Number of encoding failures.",
	}, []string{"path"})
	m.filesActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "loki_source_file_files_active_total",
		Help: "Number of active files.",
	})

	if reg != nil {
		reg.MustRegister(
			m.readBytes,
			m.totalBytes,
			m.readLines,
			m.encodingFailures,
			m.filesActive,
		)
	}

	return &m
}
