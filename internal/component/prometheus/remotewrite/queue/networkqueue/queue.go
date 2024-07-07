package networkqueue

import "github.com/prometheus/prometheus/prompb"

type Queue interface {
	// GetTimeSeries is used to grab a TimeSeries from the pool for use.
	// These time series are owned by the queue. Once Enqueued they should not be used by anything outside
	// of the Queue itself.
	GetTimeSeries() *prompb.TimeSeries
	// Enqueue will queue the data up and return once the data has been queued. Not necessarily when sent.
	Enqueue(signals []*prompb.TimeSeries) (bool, error)
}
