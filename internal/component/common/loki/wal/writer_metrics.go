package wal

import (
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
)

type WriterMetrics struct {
	lastReclaimedSegment             *prometheus.GaugeVec
	lastWrittenTimestamp             *prometheus.GaugeVec
	reclaimedOldSegmentsSpaceCounter *prometheus.CounterVec
}

func NewWriterMetrics(reg prometheus.Registerer) *WriterMetrics {
	m := &WriterMetrics{
		lastReclaimedSegment: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "loki_write",
			Subsystem: "wal_writer",
			Name:      "last_reclaimed_segment",
			Help:      "Last reclaimed segment number",
		}, []string{}),
		lastWrittenTimestamp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "loki_write",
			Subsystem: "wal_writer",
			Name:      "last_written_timestamp",
			Help:      "Latest timestamp that was written to the WAL",
		}, []string{}),
		reclaimedOldSegmentsSpaceCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "loki_write",
			Subsystem: "wal_writer",
			Name:      "reclaimed_space",
			Help:      "Number of bytes reclaimed from storage.",
		}, []string{}),
	}

	if reg != nil {
		m.lastReclaimedSegment = util.MustRegisterOrGet(reg, m.lastReclaimedSegment).(*prometheus.GaugeVec)
		m.lastWrittenTimestamp = util.MustRegisterOrGet(reg, m.lastWrittenTimestamp).(*prometheus.GaugeVec)
		m.reclaimedOldSegmentsSpaceCounter = util.MustRegisterOrGet(reg, m.reclaimedOldSegmentsSpaceCounter).(*prometheus.CounterVec)
	}

	return m
}
