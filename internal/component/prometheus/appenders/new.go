package appenders

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/storage"
)

// New returns an appropriate appender based on the number of children. mapped forces
// seriesRefMapping for a single child, for callers that may still hold refs from other
// children.
func New(children []storage.Appender, store *SeriesRefMappingStore, mapped bool, writeLatency prometheus.Histogram, samplesForwarded prometheus.Counter) storage.Appender {
	// No destination, no work to do.
	if len(children) == 0 {
		return Noop{}
	}

	// Single destination, no need to fanout.
	if len(children) == 1 && !mapped {
		return NewPassthrough(children[0], writeLatency, samplesForwarded)
	}

	return NewSeriesRefMapping(children, store, store.Generation(), writeLatency, samplesForwarded)
}
