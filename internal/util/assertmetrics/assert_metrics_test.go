package assertmetrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
)

func TestMetricValue(t *testing.T) {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "some_metric",
		Help: "A sample metric",
	})
	gauge.Set(42)

	reg := prometheus.NewRegistry()
	reg.MustRegister(gauge)

	metrics := ReadMetrics(t, reg)
	AssertValueInStr(t, metrics, "some_metric", labels.EmptyLabels(), 42)

	gauge.Set(31337)
	AssertValueInReg(t, reg, "some_metric", labels.EmptyLabels(), 31337)
}

func TestMetricValueWithLabels(t *testing.T) {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "some_metric",
		Help: "A sample metric",
		ConstLabels: prometheus.Labels{
			"foo": "bar",
			"boo": "yah",
		},
	})
	gauge.Set(42)

	reg := prometheus.NewRegistry()
	reg.MustRegister(gauge)

	AssertValueInReg(t, reg, "some_metric", labels.FromStrings("foo", "bar", "boo", "yah"), 42)
}
