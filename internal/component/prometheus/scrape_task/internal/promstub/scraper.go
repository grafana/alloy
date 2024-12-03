package promstub

import (
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
)

func NewScraper() promadapter.Scraper {
	return &scraper{}
}

type scraper struct {
}

func (s scraper) ScrapeTarget(target discovery.Target) (promadapter.Metrics, error) {
	timestamp := time.Now().UnixMilli()
	metrics := promadapter.Metrics{}
	metrics.TimeSeries = []prompb.TimeSeries{{
		Labels: toPBLabels(target.NonMetaLabels()),
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 12},
			{Timestamp: timestamp + 1, Value: 24},
			{Timestamp: timestamp + 2, Value: 48},
		},
	}, {
		Labels: toPBLabels(target.Labels()),
		Samples: []prompb.Sample{
			{Timestamp: timestamp, Value: 191},
			{Timestamp: timestamp + 1, Value: 1337},
		},
	}}

	return metrics, nil
}

func toPBLabels(labels labels.Labels) []prompb.Label {
	r := make([]prompb.Label, len(labels))
	for i, l := range labels {
		r[i] = prompb.Label{Name: l.Name, Value: l.Value}
	}
	return r
}
