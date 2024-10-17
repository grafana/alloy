package prometheus

import (
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
)

type Sample struct {
	// Ref is the hash.
	Ref       uint64
	Labels    labels.Labels
	Timestamp int64
	Value     float64
	Exemplar  exemplar.Exemplar
}

type Histogram struct {
	// Ref is the hash.
	Ref            uint64
	Labels         labels.Labels
	Timestamp      int64
	Value          float64
	Histogram      *histogram.Histogram
	FloatHistogram *histogram.FloatHistogram
	Exemplar       exemplar.Exemplar
}

type Appender interface {
	AppendSamples([]*Sample) error
	AppendHistograms([]*Histogram) error
	AppendMetadata([]metadata.Metadata) error
}
