package promstub

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/prompb"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
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
				Value: randomString(12),
			}, prompb.Label{
				Name:  "series_label",
				Value: randomString(12),
			}),
			Samples: []prompb.Sample{
				{
					Timestamp: timestamp,
					Value:     float64(num),
				},
			},
		})
	}

	simulateLatency(
		time.Microsecond*500, // min
		time.Millisecond*300, // avg
		time.Second*10,       // max
		time.Second,          // stdev
	)

	return metrics, nil
}

func toPBLabels(labels labels.Labels) []prompb.Label {
	r := make([]prompb.Label, len(labels))
	for i, l := range labels {
		r[i] = prompb.Label{Name: l.Name, Value: l.Value}
	}
	return r
}

func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}
