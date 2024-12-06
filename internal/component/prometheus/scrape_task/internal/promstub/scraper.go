package promstub

import (
	"strconv"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/random"
)

const SeriesToGenerateLabel = "__series_to_generate"

func NewScraper() promadapter.Scraper {
	return &scraper{}
}

type scraper struct {
}

func (s scraper) ScrapeTarget(target discovery.Target) (promadapter.Metrics, error) {
	timestamp := time.Now().UnixMilli()
	metrics := promadapter.Metrics{}

	sl := target.SpecificLabels([]string{SeriesToGenerateLabel})
	if len(sl) != 1 {
		return metrics, nil
	}

	num, err := strconv.Atoi(sl[0].Value)
	if err != nil {
		return metrics, nil
	}

	targetLabels := toPBLabels(target.Labels())

	for i := 0; i < num; i++ {
		metrics.TimeSeries = append(metrics.TimeSeries, prompb.TimeSeries{
			Labels: append(targetLabels, prompb.Label{
				Name:  "__name__",
				Value: random.String(12),
			}, prompb.Label{
				Name:  "series_label",
				Value: random.String(12),
			}),
			Samples: []prompb.Sample{
				{
					Timestamp: timestamp,
					Value:     float64(num),
				},
			},
		})

		// Each series adds latency, so the more series, the more latency.
		random.SimulateLatency(
			time.Nanosecond*1,    // min
			time.Nanosecond*10,   // avg
			time.Microsecond*100, // max
			time.Nanosecond*500,  // stdev
		)
	}

	return metrics, nil
}

func toPBLabels(labels labels.Labels) []prompb.Label {
	r := make([]prompb.Label, len(labels))
	for i, l := range labels {
		r[i] = prompb.Label{Name: l.Name, Value: l.Value}
	}
	return r
}
