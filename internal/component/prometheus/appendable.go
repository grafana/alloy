package prometheus

import (
	"github.com/prometheus/prometheus/model/labels"
)

type PromMetric struct {
	Value  float64
	TS     int64
	Labels labels.Labels
}
