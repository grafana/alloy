package appenders

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/storage"
)

// New returns an appropriate appender based on the number of children.
func New(children []storage.Appender, store *SeriesRefMappingStore, deadRefThreshold storage.SeriesRef, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.Appender {
	// No destination, no work to do.
	if len(children) == 0 {
		return Noop{}
	}

	// Single destination, no need to fanout.
	if len(children) == 1 {
		return NewPassthrough(children[0], deadRefThreshold, writeLatency, samplesForwarded)
	}

	return NewSeriesRefMapping(children, store, writeLatency, samplesForwarded)
}

// NewV2 returns an appropriate v2 appender based on the number of children.
func NewV2(children []storage.AppenderV2, store *SeriesRefMappingStore, deadRefThreshold storage.SeriesRef, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.AppenderV2 {
	if len(children) == 0 {
		return noopV2{}
	}

	if len(children) == 1 {
		return NewPassthroughV2(children[0], deadRefThreshold, writeLatency, samplesForwarded)
	}

	return NewSeriesRefMappingV2(children, store, writeLatency, samplesForwarded)
}
