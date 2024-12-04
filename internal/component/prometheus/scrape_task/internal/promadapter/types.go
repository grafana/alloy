package promadapter

import (
	"github.com/prometheus/prometheus/prompb"

	"github.com/grafana/alloy/internal/component/discovery"
)

type TimeSeries = prompb.TimeSeries

type Metadata = prompb.MetricMetadata

type Metrics struct {
	TimeSeries []TimeSeries
	Metadata   []Metadata
}

func (m *Metrics) SeriesCount() int {
	r := 0
	for _, ts := range m.TimeSeries {
		r += len(ts.Samples)
	}
	return r
}

type Scraper interface {
	ScrapeTarget(target discovery.Target) (Metrics, error)
}

type Sender interface {
	Send(metrics []Metrics) error
}
