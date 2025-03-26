package fsdump

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	profilesReceived prometheus.Counter
	profilesWritten  prometheus.Counter
	profilesDropped  prometheus.Counter
	bytesWritten     prometheus.Counter
	writeErrors      prometheus.Counter
	filesRemoved     prometheus.Counter
	currentSizeBytes prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) *metrics {
	m := &metrics{
		profilesReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_fsdump_profiles_received_total",
			Help: "Total number of profiles received by the fsdump component.",
		}),
		profilesWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_fsdump_profiles_written_total",
			Help: "Total number of profiles written to files by the fsdump component.",
		}),
		profilesDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_fsdump_profiles_dropped_total",
			Help: "Total number of profiles dropped by relabeling rules in the fsdump component.",
		}),
		bytesWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_fsdump_bytes_written_total",
			Help: "Total number of bytes written to files by the fsdump component.",
		}),
		writeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_fsdump_write_errors_total",
			Help: "Total number of errors encountered when writing profiles to files.",
		}),
		filesRemoved: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pyroscope_fsdump_files_removed_total",
			Help: "Total number of files removed by cleanup operations.",
		}),
		currentSizeBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pyroscope_fsdump_current_size_bytes",
			Help: "Current total size of all files in the target directory.",
		}),
	}

	if reg != nil {
		m.profilesReceived = util.MustRegisterOrGet(reg, m.profilesReceived).(prometheus.Counter)
		m.profilesWritten = util.MustRegisterOrGet(reg, m.profilesWritten).(prometheus.Counter)
		m.profilesDropped = util.MustRegisterOrGet(reg, m.profilesDropped).(prometheus.Counter)
		m.bytesWritten = util.MustRegisterOrGet(reg, m.bytesWritten).(prometheus.Counter)
		m.writeErrors = util.MustRegisterOrGet(reg, m.writeErrors).(prometheus.Counter)
		m.filesRemoved = util.MustRegisterOrGet(reg, m.filesRemoved).(prometheus.Counter)
		m.currentSizeBytes = util.MustRegisterOrGet(reg, m.currentSizeBytes).(prometheus.Gauge)
	}

	return m
}
