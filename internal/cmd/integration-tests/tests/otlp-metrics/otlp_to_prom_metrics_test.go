package main

import (
	"testing"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
)

func TestOTLPToPromMetrics(t *testing.T) {
	// Not using the default here because some metric names change during the conversion.
	metrics := []string{
		"example_counter_total",       // Change from example_counter to example_counter_total.
		"example_float_counter_total", // Change from example_float_counter to example_float_counter_total.
		"example_updowncounter",
		"example_float_updowncounter",
		"example_histogram_bucket",
		"example_float_histogram_bucket",
	}

	config := common.Config{
		T:                t,
		TestName:         "otlp_to_prom_metrics",
		Metrics:          metrics,
		HistogramMetrics: common.OtelDefaultHistogramMetrics,
		ExpectedMetadata: common.ExpectedMetadata,
	}
	common.MimirMetricsTest(config)
}
