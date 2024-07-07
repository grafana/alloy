package types

import (
	"sync"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
)

// TimeSeries represent a superset of metric types.
// TimeSeries must only be created from a pool and must be returned when done.
type TimeSeries struct {
	SeriesLabels   labels.Labels
	Value          float64
	Histogram      *histogram.Histogram
	FloatHistogram *histogram.FloatHistogram
	Timestamp      int64
	ExemplarLabels labels.Labels
	// The type of series: sample, exemplar, or histogram.
	Type SeriesType
}

var TimeSeriesPool = sync.Pool{New: func() any {
	return &TimeSeries{}
}}

type SeriesType int8

const (
	Sample SeriesType = iota
	Exemplar
	Histogram
	FloatHistogram
)
